//go:build linux

package resipnetns

import (
	"context"
	"strconv"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
)

// The ranks of the gateway of a resource, by where it comes from.
const (
	gatewayNone = iota
	gatewayOfNetwork
	gatewayOwn
)

// routeOwner returns the ip.netns resource of the object that sets the
// default route of the namespace this resource is in, for the family of its
// address, which may be this one.
//
// A namespace has one default route per family, and the ip.netns resources
// of a container with an address of that family each have a gateway to offer: their own, the one of their
// network, or none, in which case the route goes through their interface.
// The gateway a resource sets itself prevails over the one of a network,
// which prevails over none, and the lowest resource id prevails among equals,
// whatever the order the resources start in. The other resources leave the
// default route alone.
//
// A resource that cannot start, its configuration not fitting its network,
// or that the running action does not start, as one a resource selection
// leaves out, sets no route, and is no owner: the namespace would be left
// with none.
//
// The route is set only when the namespace has none, as startRoutes says:
// another resource of the namespace, an ip.cni one, may have set it.
func (t *T) routeOwner(ctx context.Context) *T {
	owner := t
	od := t.GetObjectDriver()
	if od == nil {
		return t
	}
	v6 := t.isIPv6()
	for _, r := range od.ResourcesByDrivergroups([]driver.Group{driver.GroupIP}) {
		o, ok := r.(*T)
		if !ok || o == t || o.NetNS != t.NetNS || o.IsDisabled() || o.isIPv6() != v6 || o.netErr != nil {
			continue
		}
		if selected, known := resource.IsSelected(ctx, o); known && !selected {
			continue
		}
		if o.precedes(owner) {
			owner = o
		}
	}
	return owner
}

// precedes says the resource prevails over o to set the default route.
func (t *T) precedes(o *T) bool {
	if t.gatewayRank != o.gatewayRank {
		return t.gatewayRank > o.gatewayRank
	}
	return ridLess(t.RID(), o.RID())
}

// ridLess orders the resource ids by their index, ip#2 before ip#10.
func ridLess(a, b string) bool {
	ia, erra := strconv.Atoi(ridIndex(a))
	ib, errb := strconv.Atoi(ridIndex(b))
	if erra == nil && errb == nil {
		return ia < ib
	}
	return a < b
}

func ridIndex(rid string) string {
	for i := len(rid) - 1; i >= 0; i-- {
		if rid[i] == '#' {
			return rid[i+1:]
		}
	}
	return rid
}
