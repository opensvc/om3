// Package netstatus holds what the setup of a network made on a node: the
// device it set up, the part of the network the node hands out, how the node
// reaches each other node of the network, and what failed. Each node records
// it for itself, from the decisions its setup made on what only the node
// knows, as its interface addresses, and publishes it in the cluster data.
package netstatus

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type (
	// Status is what the last setup of a network made on a node.
	Status struct {
		// Type is the type of the network.
		Type string `json:"type"`

		// Network is the address range of the network, empty for a
		// network with no address, as a l2_bridge.
		Network string `json:"network"`

		// Dev is the device of the node the network is on, as its bridge,
		// empty for a network on none.
		Dev string `json:"dev,omitempty"`

		// Subnet is the part of the network this node hands out, empty
		// when the node hands out the whole network, or none of it.
		Subnet string `json:"subnet,omitempty"`

		// Peers are the other nodes of the network the node reaches
		// itself, by a route or a tunnel, as the setup reached them.
		Peers []Peer `json:"peers,omitempty"`

		// Skipped says why the setup left the network alone, as a
		// network disabled.
		Skipped string `json:"skipped,omitempty"`

		// Error is the failure of the setup, empty when it succeeded.
		Error string `json:"error,omitempty"`

		// AppliedAt is when the setup ended, by the clock of the node.
		AppliedAt time.Time `json:"applied_at"`
	}

	// Peer is another node of a network, as this node reaches it.
	Peer struct {
		// Nodename is the name of the peer node.
		Nodename string `json:"nodename"`

		// Local is the address of this node the network reaches the peer
		// from, and Remote the address of the peer it reaches.
		Local  net.IP `json:"local"`
		Remote net.IP `json:"remote"`

		// Subnet is the part of the network the peer hands out.
		Subnet string `json:"subnet"`

		// Tunnel is the device of the tunnel carrying the network to the
		// peer, empty when the network is routed to it.
		Tunnel string `json:"tunnel"`

		// Error is why the setup could not reach the peer, empty when it
		// did.
		Error string `json:"error,omitempty"`
	}

	// Element is the status of a network on a node.
	Element struct {
		Name  string
		Node  string
		Value *Status
	}

	// Data holds the statuses of the networks of the cluster nodes.
	Data struct {
		sync.RWMutex
		data map[[2]string]*Status
	}
)

// StatusData holds the statuses of the networks on every node the daemon
// knows.
var StatusData = NewData()

// NewData returns an empty holder of network statuses.
func NewData() *Data {
	return &Data{data: make(map[[2]string]*Status)}
}

// DeepCopy returns a copy of t sharing nothing with it.
func (t *Status) DeepCopy() *Status {
	if t == nil {
		return nil
	}
	c := *t
	if t.Peers != nil {
		c.Peers = make([]Peer, len(t.Peers))
		for i, p := range t.Peers {
			c.Peers[i] = p
			c.Peers[i].Local = slices.Clone(p.Local)
			c.Peers[i].Remote = slices.Clone(p.Remote)
		}
	}
	return &c
}

// Set records the status of the network name on node.
func (t *Data) Set(name, node string, v *Status) {
	t.Lock()
	defer t.Unlock()
	t.data[[2]string{name, node}] = v
}

// Unset forgets the status of the network name on node.
func (t *Data) Unset(name, node string) {
	t.Lock()
	defer t.Unlock()
	delete(t.data, [2]string{name, node})
}

// GetAll returns the statuses of every network on every node.
func (t *Data) GetAll() []Element {
	return t.filter(func([2]string) bool { return true })
}

// GetByNode returns the statuses of the networks on node.
func (t *Data) GetByNode(node string) []Element {
	return t.filter(func(k [2]string) bool { return k[1] == node })
}

func (t *Data) filter(keep func([2]string) bool) []Element {
	t.RLock()
	defer t.RUnlock()
	l := make([]Element, 0)
	for k, v := range t.data {
		if keep(k) {
			l = append(l, Element{Name: k[0], Node: k[1], Value: v.DeepCopy()})
		}
	}
	return l
}

// suffix is the one of the files the statuses are saved in.
const suffix = ".status.json"

// Save writes the status of the network name in dir, through a temporary
// file renamed in place.
func Save(dir, name string, st Status) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	p := filepath.Join(dir, name+suffix)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Remove removes the status of the network name from dir, absent being no
// error.
func Remove(dir, name string) error {
	err := os.Remove(filepath.Join(dir, name+suffix))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Load returns the statuses saved in dir, by network name. A file that does
// not parse is skipped.
func Load(dir string) (map[string]Status, error) {
	m := make(map[string]Status)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	} else if err != nil {
		return nil, err
	}
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), suffix)
		if !ok || e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var st Status
		if err := json.Unmarshal(b, &st); err != nil {
			continue
		}
		m[name] = st
	}
	return m, nil
}
