//go:build linux

package resipnetns

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/drivers/resip"
)

// alloc is the address this resource draws from the network its network
// keyword names, as every ip driver draws it: the one it keeps on a lan
// network, see keeps.
func (t *T) alloc() *resip.Allocation {
	if t.onLan() {
		return t.kept().Alloc()
	}
	return t.drawn()
}

// drawn is the address this resource draws at each start, which also
// resolves the network.
func (t *T) drawn() *resip.Allocation {
	if t._alloc == nil {
		t._alloc = &resip.Allocation{Network: t.Network, Path: t.Path, RID: t.RID(), Log: t.Log()}
	}
	return t._alloc
}

// kept is the address this resource keeps on a lan network.
func (t *T) kept() *resip.Kept {
	if t._kept == nil {
		t._kept = resip.NewKept(t.Path, t.RID(), t.Network, t.Shared, t.GetObject(), t.Log())
	}
	return t._kept
}

// onLan says the network the address is drawn from is a segment every node
// is on, and draws from.
func (t *T) onLan() bool {
	nw, err := t.resolveNetwork()
	if err != nil || nw == nil {
		return false
	}
	_, ok := nw.(network.HostDever)
	return ok
}

// keeps says the resource keeps its address from a start to the next, until
// it is unprovisioned, as ip.host does, rather than draw one at each start
// and release it at each stop.
//
// The address of a container on a bridge or routed_bridge network is reached
// through its DNS record, which follows the address the status reports, so a
// new one at each start is no change to its clients. A lan network is a
// segment the clients of the service are on, and they know it by its address
// rather than by a record they may not be able to resolve.
func (t *T) keeps() bool {
	return t.Name == "" && t.onLan()
}

// resolveNetwork returns the om network the network keyword names.
//
// The keyword used to hold the address of the network in dotted notation,
// which set the destination of the route del_net_route removes. That
// destination is the connected route the kernel adds along with the address,
// so it is derived from the address and the mask now, and the keyword names
// the network the address is drawn from, as it does on ip.cni.
func (t *T) resolveNetwork() (network.Networker, error) {
	return t.drawn().Resolve()
}

// ipam returns the allocator of the network this resource draws from, or nil
// when it draws from none.
//
// The addresses the cluster holds on its other nodes are consulted only for a
// network whose range every node draws from: a routed_bridge gives this node
// a range of its own, and the addresses of a bridge are node local and not
// routable, so an address in use elsewhere is never one this node could hand
// out by mistake.
func (t *T) ipam() (*ipam.T, error) {
	return t.alloc().Allocator()
}

// ipamKey names the reservation of this resource.
func (t *T) ipamKey() string {
	return t.alloc().Key()
}

// allocateIP reserves the address of this resource, and returns the one it
// already holds when it holds one.
func (t *T) allocateIP(ctx context.Context) (net.IP, error) {
	return t.alloc().Allocate(ctx)
}

// allocatedIP returns the address reserved for this resource, or nil when it
// has none. It never reserves one: reading a status must not take an address.
func (t *T) allocatedIP() (net.IP, error) {
	return t.alloc().Allocated()
}

// freeIP releases the address of this resource.
func (t *T) freeIP() error {
	return t.alloc().Free()
}

// networkDev returns the device of the network this resource draws from.
func (t *T) networkDev() string {
	nw, err := t.resolveNetwork()
	if err != nil || nw == nil {
		return ""
	}
	if i, ok := nw.(interface{ BackendDevName() string }); ok {
		return i.BackendDevName()
	}
	return ""
}

// Configure fills from the network what the configuration did not say.
//
// A resource drawing its address from a network needs the device, the netmask
// and the gateway of that network, and they are the network's to know: naming
// the network is enough, and repeating them in the object configuration is a
// second copy to keep in step. An explicit value always wins.
func (t *T) Configure() error {
	nw, err := t.resolveNetwork()
	if err != nil {
		return err
	}
	if t.Gateway != "" {
		t.gatewayRank = gatewayOwn
	}
	if nw == nil {
		return nil
	}
	if i, ok := nw.(network.HostDever); ok {
		t.configureLan(nw, i)
		return nil
	}
	if t.Dev == "" {
		t.Dev = t.networkDev()
	}
	i, err := t.ipam()
	if err != nil {
		return err
	}
	if i == nil || i.Range == nil {
		// A network om draws no address from, the lo network among them, has
		// nothing this resource can be built out of. Saying so here beats the
		// device lookup failing later on an empty name.
		if t.Dev == "" && t.Name == "" {
			return fmt.Errorf("network %s provides neither a device nor an address to draw from: name a dev and a name, or name a network om allocates in", nw.Name())
		}
		return nil
	}
	if t.Netmask == "" {
		ones, _ := i.Range.Mask.Size()
		t.Netmask = fmt.Sprintf("%d", ones)
	}
	if t.Gateway == "" {
		if gw := ipam.Gateway(i.Range); gw != nil {
			t.Gateway, t.gatewayRank = gw.String(), gatewayOfNetwork
		}
	}
	return nil
}

// isLinuxBridge says a link is a linux bridge, replaced by the tests.
var isLinuxBridge = func(dev string) bool {
	_, err := os.Stat(filepath.Join("/sys/class/net", dev, "bridge"))
	return err == nil
}

// configureLan fills from a lan network what the configuration did not say:
// the interface of this node on the segment, which the link of the namespace
// is a child of, and the prefix length of the segment.
//
// The gateway is the one the network names, the router of the segment: the
// first address of the segment plus one, which a bridge network answers on,
// may be an address om hands out on a lan network.
//
// What does not fit is reported when the resource is started or its status
// read, rather than failing every load of the object.
func (t *T) configureLan(nw network.Networker, i network.HostDever) {
	if t.Addr != "" && t.kept().PerInstance && !t.kept().HasOwnAddr(t.GetObject()) {
		// The address of all the instances, as a failover object records
		// it, read here for want of one chosen for this instance: each
		// instance of a flex resource draws its own.
		t.Addr = ""
	}
	switch {
	case t.Mode == "ipvlan-l3" || t.Mode == "ipvlan-l3s":
		t.netErr = fmt.Errorf("mode %s routes the address through this node, where the hosts of the segment of network %s do not look for it: use macvlan or ipvlan-l2", t.Mode, nw.Name())
		return
	case t.Mode == "dedicated" || t.Tags.Has(tagDedicated):
		// The interface of this node on the segment is what the node
		// itself is reached by, and moving it into the namespace takes it
		// away from the node.
		if t.Dev == "" {
			t.netErr = fmt.Errorf("mode dedicated moves dev into the namespace, and network %s names no interface but the one of this node on the segment: set dev to an interface of its own", nw.Name())
			return
		}
		if dev, err := i.HostDev(); err == nil && dev == t.Dev {
			t.netErr = fmt.Errorf("mode dedicated would move %s, the interface of this node on the segment of network %s, into the namespace: set dev to an interface of its own", dev, nw.Name())
			return
		}
	case t.Dev == "":
		t.Dev, t.netErr = i.HostDev()
		if t.netErr != nil {
			return
		}
	}
	if t.Mode == "bridge" && !isLinuxBridge(t.Dev) {
		// The veth of the namespace is plugged into dev, which only a
		// bridge takes. The interface of the node on the segment is one
		// when the node is reached through a bridge, and a physical one
		// otherwise.
		t.netErr = fmt.Errorf("mode bridge plugs the namespace into a bridge on the segment of network %s, and %s is not one: set dev to a bridge on the segment, or use macvlan", nw.Name(), t.Dev)
		return
	}
	if t.Gateway == "" {
		if g, ok := nw.(network.Gatewayer); ok {
			if gw, err := g.Gateway(); err != nil {
				t.netErr = err
				return
			} else if gw != nil {
				t.Gateway, t.gatewayRank = gw.String(), gatewayOfNetwork
			}
		}
	}
	if t.Netmask == "" {
		if m, ok := nw.(network.Netmasker); ok {
			if n, err := m.Netmask(); err == nil {
				t.Netmask = fmt.Sprint(n)
			} else {
				t.netErr = err
			}
		}
	}
}
