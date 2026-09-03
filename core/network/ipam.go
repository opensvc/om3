package network

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/hostname"
)

// ipAddrInfoKey is the resource status info an ip resource publishes its
// address under.
const ipAddrInfoKey = "ipaddr"

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
	if len(reservations) == 0 {
		return nil
	}
	nodename := hostname.Hostname()
	for _, nw := range nws {
		i, ok := nw.(IPAMer)
		if !ok {
			continue
		}
		rng, err := i.AllocatableRange(nodename)
		if err != nil {
			nw.Log().Warnf("ipam: %s", err)
			continue
		}
		if rng == nil {
			continue
		}
		a := &ipam.T{
			Name:    nw.Name(),
			Range:   rng,
			Gateway: ipam.Gateway(rng),
			Dir:     ipam.StoreDir(nw.Name()),
		}
		adopted, err := a.Adopt(reservations)
		if err != nil {
			return err
		}
		if adopted > 0 {
			nw.Log().Infof("ipam: adopted %d address(es) already held in this network", adopted)
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
