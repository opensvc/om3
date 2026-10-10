package resip

import (
	"context"
	"fmt"
	"net"

	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/plog"
)

// Kept is the address a resource draws from a network and keeps from a start
// to the next, until it is unprovisioned: the address is the service's, which
// its clients know it by.
//
// The address of all the instances is recorded in the addr keyword of the
// configuration, which every node holds, whatever became of the node that
// drew it: a node the service fails over to after the others crashed reads
// it there, rather than draw again from a cluster that forgot them. The
// address of an instance, when each draws its own, is kept in the reservation
// store of its node.
type Kept struct {
	Path        naming.Path
	RID         string
	Network     string
	PerInstance bool
	Log         *plog.Logger

	alloc *Allocation
}

// NewKept returns the address a resource keeps. The object says whether each
// instance draws its own.
func NewKept(p naming.Path, rid, network string, shared bool, obj any, log *plog.Logger) *Kept {
	return &Kept{
		Path:        p,
		RID:         rid,
		Network:     network,
		PerInstance: PerInstance(shared, obj),
		Log:         log,
	}
}

// PerInstance says each instance draws an address of its own, which is the
// case of a resource not shared of a flex object: its instances run at once,
// and an address they all brought up would be one address on several nodes.
//
// A failover object runs one instance at a time, which takes the address
// along when it moves. A shared resource of a flex object is one address for
// all its instances, should that be the point.
func PerInstance(shared bool, obj any) bool {
	if shared {
		return false
	}
	o, ok := obj.(interface{ Topology() topology.T })
	return ok && o.Topology() == topology.Flex
}

// Alloc returns the reservation of the address, the one of this instance when
// each instance draws its own, apart from the ones of the other nodes.
func (t *Kept) Alloc() *Allocation {
	if t.alloc == nil {
		rid := t.RID
		if t.PerInstance {
			rid += "@" + hostname.Hostname()
		}
		t.alloc = &Allocation{Network: t.Network, Path: t.Path, RID: rid, Log: t.Log}
	}
	return t.alloc
}

// AddrKey is where the address drawn is recorded: addr, the same on every
// node, or addr@<node>, the address root chose for this instance when each
// instance draws its own.
func (t *Kept) AddrKey() key.T {
	if t.PerInstance {
		return key.New(t.RID, "addr@"+hostname.Hostname())
	}
	return key.New(t.RID, "addr")
}

// HasOwnAddr says the configuration records an address for this instance
// under its own key, rather than one the evaluation fell back on.
func (t *Kept) HasOwnAddr(obj any) bool {
	o, ok := obj.(object.Configurer)
	return ok && o.Config().HasKey(t.AddrKey())
}

// Reserve returns the address of the resource, reserved on this node: addr,
// the one the configuration says it drew, or else one drawn now and written
// to the configuration. It also returns the address the resource gave up for
// it, nil when it gave none up: the one this node held for it before a redraw,
// or before addr was changed.
//
// A configuration with no address while this node holds one for the resource
// had it unset, since om writes it as it draws and unsets it as it releases.
// Unsetting it is asking for another address, so the one held is given up
// rather than drawn again, which the draw would do, keyed as it is on the
// resource.
func (t *Kept) Reserve(ctx context.Context, addr string) (net.IP, net.IP, error) {
	if addr != "" {
		ip := net.ParseIP(addr)
		if ip == nil {
			return nil, nil, fmt.Errorf("addr %q is not an ip address", addr)
		}
		previous, err := t.Alloc().Reserve(ip)
		if err != nil {
			return nil, nil, err
		}
		return ip, previous, nil
	}
	if t.PerInstance {
		// The address of this instance is kept in the reservation store of
		// this node, which holds it from a start to the next until the
		// instance is unprovisioned. Recording it in the configuration, as
		// the address of all the instances is, would have the instances
		// starting at once write it at once, each over the others.
		ip, err := t.Alloc().Allocate(ctx)
		return ip, nil, err
	}
	previous, err := t.Alloc().Allocated()
	if err != nil {
		return nil, nil, err
	}
	var ip net.IP
	if previous == nil {
		ip, err = t.Alloc().Allocate(ctx)
	} else {
		ip, err = t.Alloc().Redraw(ctx, previous)
	}
	if err != nil {
		return nil, nil, err
	}
	if err := t.Record(ctx, "", ip.String()); err != nil {
		return nil, nil, fmt.Errorf("record the address %s drawn from network %s: %w", ip, t.Network, err)
	}
	return ip, previous, nil
}

// Release releases the reservation of this node, and, on the leader, removes
// from the configuration addr, the address of all the instances, once for
// all of them.
func (t *Kept) Release(ctx context.Context, addr string, leader bool) error {
	if err := t.Alloc().Free(); err != nil {
		return err
	}
	if addr == "" || t.PerInstance {
		// An address of one instance is not recorded: an addr@<node> is
		// one root chose, and stays.
		return nil
	}
	if !leader {
		return nil
	}
	return t.Record(ctx, addr, "")
}

// Record writes the address drawn from the network in the configuration of
// the object, which the daemon brings to the other nodes, and unsets the one
// recorded, previous, when addr is empty.
func (t *Kept) Record(ctx context.Context, previous, addr string) error {
	obj, err := object.NewConfigurer(t.Path)
	if err != nil {
		return err
	}
	k := t.AddrKey()
	if addr == "" {
		if err := obj.Unset(ctx, k); err != nil {
			return err
		}
		t.Log.Infof("address %s drawn from network %s removed from the configuration", previous, t.Network)
		return nil
	}
	if err := obj.Set(ctx, keyop.T{Key: k, Op: keyop.Set, Value: addr}); err != nil {
		return err
	}
	t.Log.Infof("address %s drawn from network %s recorded in the configuration", addr, t.Network)
	return nil
}

// Duplicates returns the resources of the other nodes the peer records say
// hold ip, the address of this one, as "<key> on <node>".
//
// The address of a failover resource is the same on every node, a stopped
// instance reporting it too, which is no duplicate. Another resource holding
// it, or another instance of a resource drawing per instance, is: two halves
// of a cluster not hearing each other draw from the same range blind to each
// other, and the duplicate is for an administrator to clean up.
func (t *Kept) Duplicates(ip net.IP) []string {
	if ip == nil || t.Network == "" {
		return nil
	}
	records, err := ipam.ReadPeerRecords(ipam.PeerDir(t.Network))
	if err != nil {
		return nil
	}
	k := ipam.Key(t.Path, t.RID)
	l := make([]string, 0)
	for _, record := range records {
		if !record.IP.Equal(ip) || record.Node == hostname.Hostname() {
			continue
		}
		if record.Key == k && !t.PerInstance {
			continue
		}
		l = append(l, fmt.Sprintf("%s on %s", record.Key, record.Node))
	}
	return l
}
