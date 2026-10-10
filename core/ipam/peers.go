package ipam

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type (
	// PeerRecord is an address another node reported one of its instances
	// holds in a network every node draws from.
	PeerRecord struct {
		IP   net.IP
		Node string
		Key  string
	}
)

// PeerDir is where a node records the addresses the other nodes reported
// holding in a network every node draws from, one file per address naming the
// node and the reservation key.
//
// The draws read the stores of the nodes alive, and the status the cluster
// keeps of the others, which it drops when it stops hearing from them, and
// forgets when the node restarts. What a node down holds stays on its
// interfaces all the same: the records are what this node remembers of it
// for as long as it is not told otherwise.
func PeerDir(name string) string {
	return StoreDir(name) + ".peers"
}

// ReadPeerRecords returns the records of a peer directory.
func ReadPeerRecords(dir string) ([]PeerRecord, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	l := make([]PeerRecord, 0, len(entries))
	for _, entry := range entries {
		ip := net.ParseIP(entry.Name())
		if ip == nil {
			continue
		}
		record, ok := readPeerRecord(dir, entry.Name())
		if !ok {
			continue
		}
		record.IP = ip
		l = append(l, record)
	}
	return l, nil
}

func readPeerRecord(dir, name string) (PeerRecord, bool) {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return PeerRecord{}, false
	}
	node, key, ok := strings.Cut(strings.TrimSpace(string(b)), " ")
	if !ok {
		return PeerRecord{}, false
	}
	return PeerRecord{Node: node, Key: key}, true
}

// SetPeerRecords makes the records of an object on a node the addresses
// given, by address, the key each is held for: the ones it no longer reports
// are removed, and the others written. It returns how many it wrote and
// removed.
func SetPeerRecords(dir, node, path string, addrs map[string]string) (int, int, error) {
	records, err := ReadPeerRecords(dir)
	if err != nil {
		return 0, 0, err
	}
	written, removed := 0, 0
	for _, record := range records {
		p, ok := PathOfKey(record.Key)
		if record.Node != node || !ok || p.String() != path {
			continue
		}
		if key, ok := addrs[record.IP.String()]; ok && key == record.Key {
			delete(addrs, record.IP.String())
			continue
		}
		if err := os.Remove(filepath.Join(dir, record.IP.String())); err != nil && !os.IsNotExist(err) {
			return written, removed, err
		}
		removed++
	}
	if len(addrs) == 0 {
		return written, removed, nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return written, removed, err
	}
	for addr, key := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if err := writePeerRecord(dir, ip.String(), node, key); err != nil {
			return written, removed, err
		}
		written++
	}
	return written, removed, nil
}

// writePeerRecord writes a record whole, so a reader never reads half of
// one.
func writePeerRecord(dir, name, node, key string) error {
	tmp, err := os.CreateTemp(dir, "."+name+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := fmt.Fprintf(tmp, "%s %s\n", node, key); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, name))
}

// DropPeerRecords removes the records keep says no to, and returns how many.
func DropPeerRecords(dir string, keep func(PeerRecord) bool) (int, error) {
	records, err := ReadPeerRecords(dir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, record := range records {
		if keep(record) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, record.IP.String())); err != nil && !os.IsNotExist(err) {
			return n, err
		}
		n++
	}
	return n, nil
}
