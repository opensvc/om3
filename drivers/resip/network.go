package resip

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
)

// Allocation is the address an ip resource draws from the om network its
// network keyword names.
type Allocation struct {
	// Network is the value of the network keyword.
	Network string

	// Path and RID name the resource, which the address is reserved for.
	Path naming.Path
	RID  string

	Log *plog.Logger

	resolved bool
	nw       network.Networker
}

// Resolve returns the om network the network keyword names, nil when it
// names none.
//
// The keyword used to hold the address of the network in dotted notation. A
// value that is still an address is therefore obsolete rather than wrong: it
// is reported and ignored. A value that is neither an address nor a network
// is a mistake worth stopping for, a renamed network or a typo.
func (t *Allocation) Resolve() (network.Networker, error) {
	if t.resolved {
		return t.nw, nil
	}
	t.resolved = true
	if t.Network == "" {
		return nil, nil
	}
	nw, names, err := network.Lookup(t.Network)
	if err != nil {
		return nil, err
	}
	if nw != nil {
		t.nw = nw
		return nw, nil
	}
	if IsAddr(t.Network) {
		t.Log.Warnf("the network keyword holds the address %s, which is obsolete and ignored: the keyword names the om network the address is drawn from now", t.Network)
		return nil, nil
	}
	return nil, fmt.Errorf("unknown network %s, expected one of %s", t.Network, strings.Join(names, ", "))
}

// IsAddr reports whether a value is an address or a subnet, which is what the
// network keyword used to hold.
func IsAddr(s string) bool {
	if net.ParseIP(s) != nil {
		return true
	}
	_, _, err := net.ParseCIDR(s)
	return err == nil
}

// Allocator returns the allocator of the network the resource draws from, nil
// when it draws from none.
func (t *Allocation) Allocator() (*ipam.T, error) {
	nw, err := t.Resolve()
	if err != nil {
		return nil, err
	}
	if nw == nil {
		return nil, nil
	}
	return network.NewAllocator(nw, hostname.Hostname())
}

// Key names the reservation of the resource. An instance holds as many ip
// resources as it needs, several of them in one network, so the address
// belongs to the resource rather than to the object.
func (t *Allocation) Key() string {
	return ipam.Key(t.Path, t.RID)
}

// Allocate reserves the address of the resource, and returns the one it
// already holds when it holds one.
func (t *Allocation) Allocate(ctx context.Context) (net.IP, error) {
	i, err := t.Allocator()
	if err != nil || i == nil {
		return nil, err
	}
	ip, err := network.AllocateFor(ctx, i, t.Path, t.RID)
	if err != nil {
		return nil, err
	}
	t.Log.Infof("allocated %s in network %s", ip, i.Name)
	return ip, nil
}

// Allocated returns the address reserved for the resource, nil when it has
// none. It never reserves one: reading a status must not take an address.
func (t *Allocation) Allocated() (net.IP, error) {
	i, err := t.Allocator()
	if err != nil || i == nil {
		return nil, err
	}
	return i.Allocated(t.Key())
}

// Free releases the address of the resource.
func (t *Allocation) Free() error {
	i, err := t.Allocator()
	if err != nil || i == nil {
		return err
	}
	return i.Free(t.Key())
}

// Reserve records ip as the address of the resource on this node, the one its
// configuration says it drew on another node or earlier: the address is the
// resource's, whichever node drew it. A reservation of another address for
// the resource is released, and an address another resource holds on this
// node is refused.
func (t *Allocation) Reserve(ip net.IP) error {
	i, err := t.Allocator()
	if err != nil || i == nil {
		return err
	}
	if i.Range == nil || !i.Range.Contains(ip) {
		return fmt.Errorf("%s is not an address of network %s (%s): unset the addr keyword to draw one", ip, i.Name, i.Range)
	}
	key := t.Key()
	held, err := i.Allocated(key)
	if err != nil {
		return err
	}
	if held != nil && held.Equal(ip) {
		return nil
	}
	if held != nil {
		if err := i.Free(key); err != nil {
			return err
		}
	}
	if _, err := i.Adopt([]ipam.Reservation{{IP: ip, Key: key}}); err != nil {
		return err
	}
	if got, err := i.Allocated(key); err != nil {
		return err
	} else if got == nil || !got.Equal(ip) {
		return fmt.Errorf("%s is reserved on this node for another resource than %s", ip, key)
	}
	return nil
}
