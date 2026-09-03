package network

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/hostname"
)

// ipAddrInfoKey is the resource status info an ip resource publishes its
// address under.
const ipAddrInfoKey = "ipaddr"

// NewAllocator returns the allocator of a network on a node, or nil when the
// network is one om allocates no address in.
func NewAllocator(nw Networker, nodename string) (*ipam.T, error) {
	i, ok := nw.(IPAMer)
	if !ok {
		return nil, nil
	}
	rng, err := i.AllocatableRange(nodename)
	if err != nil {
		return nil, fmt.Errorf("network %s: %w", nw.Name(), err)
	}
	if rng == nil {
		return nil, nil
	}
	return &ipam.T{
		Name:    nw.Name(),
		Range:   rng,
		Gateway: ipam.Gateway(rng),
		Dir:     ipam.StoreDir(nw.Name()),
		// The record of the host-local plugin is read while it still hands
		// out addresses of this network, and never written: an address it
		// gave has no reservation of om's until a setup adopts it.
		PeerDirs: []string{filepath.Join(cniCacheDir, nw.Name())},
	}, nil
}

// Lookup returns the network of a name, or nil when no network has it.
func Lookup(name string) (Networker, []string, error) {
	node, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return nil, nil, err
	}
	names := make([]string, 0)
	for _, nw := range Networks(node) {
		if nw.Name() == name {
			return nw, nil, nil
		}
		names = append(names, nw.Name())
	}
	return nil, names, nil
}

// cniCacheDir is where the host-local plugin records the addresses it hands
// out.
const cniCacheDir = "/var/lib/cni/networks"

// setupIPAM records the addresses the resources of this node already hold, so
// the allocator hands out the addresses that are free rather than the ones it
// has no reservation for yet.
//
// It runs here rather than when a resource starts, because it has to be done
// once for a whole network and before the first allocation in it. A setup is
// where the once-per-network work of a node already happens, and it runs on
// daemon start and on every cluster configuration change.
//
// This matters most the first time a node allocates in a network whose
// addresses another allocator was handing out. The host-local plugin stops
// running then, so it stops releasing what it gave: every address it holds
// would be blocked for as long as its record lasts, and every address it holds
// would be free to hand out twice if that record were ignored. Neither, once
// the addresses in use are reservations of om's.
func setupIPAM(nws []Networker) error {
	reservations, err := localReservations()
	if err != nil {
		return err
	}
	installed, err := installedPaths()
	if err != nil {
		return err
	}
	nodename := hostname.Hostname()
	for _, nw := range nws {
		a, err := NewAllocator(nw, nodename)
		if err != nil {
			nw.Log().Warnf("ipam: %s", err)
			continue
		}
		if a == nil {
			continue
		}
		adopted, err := a.Adopt(reservations)
		if err != nil {
			return err
		}
		if adopted > 0 {
			nw.Log().Infof("ipam: adopted %d address(es) already held in this network", adopted)
		}
		reaped, err := a.Reap(func(key string) bool {
			p, ok := ipam.PathOfKey(key)
			if !ok {
				// A key of a shape this om does not write is not one it may
				// decide is gone.
				return true
			}
			return installed[p.String()]
		})
		if err != nil {
			return err
		}
		if reaped > 0 {
			nw.Log().Infof("ipam: released %d address(es) held for an object that no longer exists", reaped)
		}
		drained, left, err := a.DrainPeers()
		if err != nil {
			return err
		}
		if drained > 0 {
			nw.Log().Infof("ipam: dropped %d address(es) from the record of the plugin that used to allocate them", drained)
		}
		if left > 0 {
			nw.Log().Warnf("ipam: the record of the plugin that used to allocate in this network still holds %d address(es) om accounts for in no way, and they stay excluded. Remove them from %s once nothing uses them", left, filepath.Join(cniCacheDir, nw.Name()))
		}
	}
	return nil
}

// localReservations returns the addresses the ip resources of this node hold,
// read from the status every object caches locally.
//
// The cache of the host-local plugin cannot serve: it names the holder of an
// address by the pid of a network namespace, which says nothing about which
// resource that is, so an address adopted from it could never be released by
// the resource that stops. The object status names the resource, and it is on
// this node, which is the only node whose addresses matter here.
func localReservations() ([]ipam.Reservation, error) {
	paths, err := naming.InstalledPaths()
	if err != nil {
		return nil, err
	}
	l := make([]ipam.Reservation, 0)
	for _, p := range paths {
		status, err := loadInstanceStatus(p)
		if err != nil {
			// An object with no status yet holds no address yet.
			continue
		}
		for rid, rstat := range status.Resources {
			s, ok := rstat.Info[ipAddrInfoKey].(string)
			if !ok || s == "" {
				continue
			}
			ip := net.ParseIP(s)
			if ip == nil {
				continue
			}
			l = append(l, ipam.Reservation{IP: ip, Key: ipam.Key(p, rid)})
		}
	}
	return l, nil
}

// loadInstanceStatus reads the status an object cached, without evaluating it.
func loadInstanceStatus(p naming.Path) (instance.Status, error) {
	var data instance.Status
	b, err := os.ReadFile(filepath.Join(p.VarDir(), "status.json"))
	if err != nil {
		return data, err
	}
	err = json.Unmarshal(b, &data)
	return data, err
}

// installedPaths returns the objects configured on this node, by path.
func installedPaths() (map[string]bool, error) {
	paths, err := naming.InstalledPaths()
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(paths))
	for _, p := range paths {
		m[p.String()] = true
	}
	return m, nil
}
