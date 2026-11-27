package object

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/util/getaddr"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/wakeonlan"
)

// wakeLookup, wakeBroadcastFor and wakeReceiving are replaced in tests.
var (
	wakeLookup       = getaddr.Lookup
	wakeBroadcastFor = wakeonlan.BroadcastFor
	wakeReceiving    = (*Node).wakeReceiving
)

// Wake wakes the cluster node nodename from the local node.
//
// It asks the collector first, for a virtual machine node: the collector
// queues the start of the container of its hypervisor running it, found by
// the machine uuid the node reported. When the collector can not, as for a
// physical node, it sends the magic packets configured in the
// wake#<nodename> section. The macs, when set, replace the
// wake#<nodename>.mac addresses.
//
// Without wake#<nodename>.broadcast, the packet goes to the broadcast address
// of the local interface on the subnet of the address nodename resolves to,
// the cached one when the dns does not answer.
//
// Waking the local node does nothing: it is awake. Nor does waking a node
// whose heartbeat the local daemon receives, or one the collector says is up:
// a wake queued while it was down may run once it is back, on a relay just
// back too.
func (t *Node) Wake(nodename string, macs []string) error {
	if nodename == "" {
		return fmt.Errorf("node name is not set")
	}
	if nodename == hostname.Hostname() {
		t.log.Infof("node %s is the local node: already awake", nodename)
		return nil
	}
	nodenames, err := t.Nodes()
	if err != nil {
		return err
	}
	if !slices.Contains(nodenames, nodename) {
		return fmt.Errorf("node %s is not a cluster node", nodename)
	}
	// With no daemon to ask, as a wake run while it starts, the wake goes on:
	// the collector, asked next, still knows a node up.
	if receiving, err := wakeReceiving(t, nodename); err != nil {
		t.log.Infof("wake node %s: can not read its heartbeat: %s", nodename, err)
	} else if receiving {
		t.log.Infof("node %s is already awake: its heartbeat beats", nodename)
		return nil
	}
	section := "wake#" + nodename
	if len(macs) == 0 {
		macs = t.wakeList(section, "mac")
	}
	ans, collectorErr := wakeByCollector(t, nodename)
	if collectorErr == nil {
		if ans.Awake {
			t.log.Infof("node %s is already awake: the collector says it is up", nodename)
			return nil
		}
		if ans.Queued {
			t.log.Infof("wake node %s: the collector queued the start of %s %s on %s (action %d)", nodename, ans.Path, ans.RID, ans.Hypervisor, ans.ActionID)
		} else {
			t.log.Infof("wake node %s: the start of %s %s on %s is already queued (action %d)", nodename, ans.Path, ans.RID, ans.Hypervisor, ans.ActionID)
		}
		return nil
	}
	if len(macs) == 0 {
		return fmt.Errorf("node %s: the collector can not wake it: %w, and it has no mac address to send a magic packet to: set %s.mac", nodename, collectorErr, section)
	}
	t.log.Infof("wake node %s: the collector can not wake it: %s: send magic packets", nodename, collectorErr)
	ports, err := t.wakePorts(section)
	if err != nil {
		return err
	}
	broadcasts := t.wakeList(section, "broadcast")
	if len(broadcasts) == 0 {
		broadcast, err := t.wakeBroadcast(nodename)
		if err != nil {
			return fmt.Errorf("%w: set %s.broadcast", err, section)
		}
		broadcasts = []string{broadcast}
	}
	t.log.Infof("wake node %s", nodename)
	return t.Wol(macs, broadcasts, ports)
}

// wakeReceiving returns whether the local daemon receives the heartbeat of
// nodename on any of its rx streams.
func (t *Node) wakeReceiving(nodename string) (bool, error) {
	c, err := client.New()
	if err != nil {
		return false, err
	}
	b, err := c.NewGetClusterStatus().Get()
	if err != nil {
		return false, err
	}
	// The heartbeat of the local node alone, the part of the cluster status
	// read: the full one would tie this package to the one decoding it.
	var data struct {
		Daemon struct {
			Nodename string `json:"nodename"`
		} `json:"daemon"`
		Cluster struct {
			Node map[string]struct {
				Daemon struct {
					Heartbeat daemonsubsystem.Heartbeat `json:"heartbeat"`
				} `json:"daemon"`
			} `json:"node"`
		} `json:"cluster"`
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return false, fmt.Errorf("decode the cluster status: %w", err)
	}
	hb := data.Cluster.Node[data.Daemon.Nodename].Daemon.Heartbeat
	return hb.IsReceiving(nodename), nil
}

func (t *Node) wakePorts(section string) ([]int, error) {
	var ports []int
	for _, s := range t.wakeList(section, "port") {
		port, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("%s.port: invalid port %q", section, s)
		}
		ports = append(ports, port)
	}
	return ports, nil
}

func (t *Node) wakeBroadcast(nodename string) (string, error) {
	ip, age, err := wakeLookup(nodename)
	if err != nil {
		return "", fmt.Errorf("resolve node %s address: %w", nodename, err)
	}
	if age > 0 {
		t.log.Infof("node %s address %s from the cache, %s old", nodename, ip, age)
	}
	broadcast, err := wakeBroadcastFor(ip)
	if err != nil {
		return "", fmt.Errorf("node %s address %s: %w", nodename, ip, err)
	}
	return broadcast.String(), nil
}

// wakeList returns the words of a wake#<nodename> list keyword. Commas
// separate them too, as in the om node wol and om node wake flags: a mac
// address, a broadcast address and a port have none.
func (t *Node) wakeList(section, option string) []string {
	return strings.FieldsFunc(t.mergedConfig.GetString(key.New(section, option)), func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
}
