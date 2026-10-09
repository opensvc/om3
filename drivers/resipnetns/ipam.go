//go:build linux

package resipnetns

import (
	"context"
	"fmt"
	"net"

	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/drivers/resip"
)

// alloc is the address this resource draws from the network its network
// keyword names, as every ip driver draws it.
func (t *T) alloc() *resip.Allocation {
	if t._alloc == nil {
		t._alloc = &resip.Allocation{Network: t.Network, Path: t.Path, RID: t.RID(), Log: t.Log()}
	}
	return t._alloc
}

// resolveNetwork returns the om network the network keyword names.
//
// The keyword used to hold the address of the network in dotted notation,
// which set the destination of the route del_net_route removes. That
// destination is the connected route the kernel adds along with the address,
// so it is derived from the address and the mask now, and the keyword names
// the network the address is drawn from, as it does on ip.cni.
func (t *T) resolveNetwork() (network.Networker, error) {
	return t.alloc().Resolve()
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
	if nw == nil {
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
			t.Gateway = gw.String()
		}
	}
	return nil
}
