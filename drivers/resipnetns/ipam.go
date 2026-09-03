//go:build linux

package resipnetns

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/hostname"
)

// cniCacheDir is where the host-local plugin records the addresses it hands
// out. It is read and never written: while ip.cni is served by that plugin,
// an address it gave has no reservation of om's, and would be handed out
// twice.
const cniCacheDir = "/var/lib/cni/networks"

// resolveNetwork returns the om network the network keyword names.
//
// The keyword used to hold the address of the network in dotted notation,
// which set the destination of the route del_net_route removes. That
// destination is the connected route the kernel adds along with the address,
// so it is derived from the address and the mask now, and the keyword names
// the network the address is drawn from, as it does on ip.cni.
//
// A value that is still an address is therefore obsolete rather than wrong:
// it is reported and ignored. A value that is neither an address nor a
// network is a mistake worth stopping for, a renamed network or a typo.
func (t *T) resolveNetwork() (network.Networker, error) {
	if t._networkResolved {
		return t._network, nil
	}
	t._networkResolved = true
	if t.Network == "" {
		return nil, nil
	}
	node, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0)
	for _, nw := range network.Networks(node) {
		if nw.Name() == t.Network {
			t._network = nw
			return nw, nil
		}
		names = append(names, nw.Name())
	}
	if isAddr(t.Network) {
		t.Log().Warnf("the network keyword holds the address %s, which is obsolete and ignored: the route del_net_route removes is derived from the address and the netmask. The keyword names the network the address is drawn from now", t.Network)
		return nil, nil
	}
	return nil, fmt.Errorf("unknown network %s, expected one of %s", t.Network, strings.Join(names, ", "))
}

// isAddr reports whether a value is an address or a subnet, which is what the
// network keyword used to hold.
func isAddr(s string) bool {
	if net.ParseIP(s) != nil {
		return true
	}
	_, _, err := net.ParseCIDR(s)
	return err == nil
}

// ipam returns the allocator of the network this resource draws from, or nil
// when it draws from none.
//
// The addresses the cluster holds on its other nodes are not consulted: a
// routed_bridge gives this node a range of its own, and the addresses of a
// bridge are node local and not routable, so an address in use elsewhere is
// never one this node could hand out by mistake.
func (t *T) ipam() (*ipam.T, error) {
	nw, err := t.resolveNetwork()
	if err != nil {
		return nil, err
	}
	if nw == nil {
		return nil, nil
	}
	i, ok := nw.(network.IPAMer)
	if !ok {
		return nil, fmt.Errorf("network %s allocates no address", nw.Name())
	}
	rng, err := i.AllocatableRange(hostname.Hostname())
	if err != nil {
		return nil, fmt.Errorf("network %s: %w", nw.Name(), err)
	}
	return &ipam.T{
		Name:     nw.Name(),
		Range:    rng,
		Gateway:  ipam.Gateway(rng),
		Dir:      filepath.Join(rawconfig.Paths.Var, "ipam", nw.Name()),
		PeerDirs: []string{filepath.Join(cniCacheDir, nw.Name())},
	}, nil
}

// ipamKey names the reservation of this resource. An instance holds as many ip
// resources as it needs, several of them in one network, so the address
// belongs to the resource rather than to the object.
func (t *T) ipamKey() string {
	return ipam.Key(t.Path, t.RID())
}

// allocateIP reserves the address of this resource, and returns the one it
// already holds when it holds one.
func (t *T) allocateIP() (net.IP, error) {
	i, err := t.ipam()
	if err != nil {
		return nil, err
	}
	if i == nil {
		return nil, nil
	}
	ip, err := i.Allocate(t.ipamKey())
	if err != nil {
		return nil, err
	}
	t.Log().Infof("allocated %s in network %s", ip, i.Name)
	return ip, nil
}

// allocatedIP returns the address reserved for this resource, or nil when it
// has none. It never reserves one: reading a status must not take an address.
func (t *T) allocatedIP() (net.IP, error) {
	i, err := t.ipam()
	if err != nil {
		return nil, err
	}
	if i == nil {
		return nil, nil
	}
	return i.Allocated(t.ipamKey())
}

// freeIP releases the address of this resource.
func (t *T) freeIP() error {
	i, err := t.ipam()
	if err != nil {
		return err
	}
	if i == nil {
		return nil
	}
	return i.Free(t.ipamKey())
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
	if err != nil || i == nil || i.Range == nil {
		return err
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
