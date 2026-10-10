//go:build linux

package resipnetns

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/util/plog"
)

// fakeObject is the object of the resources, which lists them.
type fakeObject struct {
	resources resource.Drivers
}

func (t fakeObject) Path() naming.Path                   { return naming.Path{Name: "svc1", Kind: naming.KindSvc} }
func (t fakeObject) Log() *plog.Logger                   { return plog.NewLogger(zerolog.New(io.Discard)) }
func (t fakeObject) VarDir() string                      { return "" }
func (t fakeObject) ResourceByID(string) resource.Driver { return nil }
func (t fakeObject) ResourcesByDrivergroups([]driver.Group) resource.Drivers {
	return t.resources
}

func newRouteTestResource(t *testing.T, rid, netns string, rank int) *T {
	t.Helper()
	r := &T{NetNS: netns, gatewayRank: rank, _ipaddr: net.ParseIP("fd01::5")}
	id, err := resourceid.Parse(rid)
	require.NoError(t, err)
	r.ResourceID = id
	return r
}

// One resource sets the default route of a namespace: an own gateway over a
// gateway of the network over none, then the lowest resource id, whatever
// the order the resources are listed or started in. A resource of another
// namespace has no say.
func TestRouteOwner(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ranks map[string]int
		want  string
	}{
		{name: "the lowest id among equals", ranks: map[string]int{"ip#10": gatewayNone, "ip#2": gatewayNone}, want: "ip#2"},
		{name: "a gateway of the network over none", ranks: map[string]int{"ip#0": gatewayNone, "ip#1": gatewayOfNetwork}, want: "ip#1"},
		{name: "an own gateway over the one of the network", ranks: map[string]int{"ip#0": gatewayOfNetwork, "ip#5": gatewayOwn, "ip#3": gatewayNone}, want: "ip#5"},
		{name: "the lowest id among own gateways", ranks: map[string]int{"ip#7": gatewayOwn, "ip#4": gatewayOwn}, want: "ip#4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj := &fakeObject{}
			l := make([]*T, 0)
			for rid, rank := range tc.ranks {
				r := newRouteTestResource(t, rid, "container#0", rank)
				obj.resources = append(obj.resources, r)
				l = append(l, r)
			}
			other := newRouteTestResource(t, "ip#99", "container#1", gatewayOwn)
			obj.resources = append(obj.resources, other)
			for _, r := range append(l, other) {
				r.SetObject(obj)
			}
			for _, r := range l {
				assert.Equal(t, tc.want, r.routeOwner(context.Background()).RID(), "seen from %s", r.RID())
			}
			assert.Equal(t, "ip#99", other.routeOwner(context.Background()).RID())
		})
	}
}

// The default routes of the two families are apart: an ipv4 resource sets the
// ipv4 one, whatever the ipv6 resources of the namespace offer.
func TestRouteOwnerByFamily(t *testing.T) {
	obj := &fakeObject{}
	v6 := newRouteTestResource(t, "ip#0", "container#0", gatewayOwn)
	v4 := newRouteTestResource(t, "ip#1", "container#0", gatewayOfNetwork)
	v4._ipaddr = net.ParseIP("10.0.0.5")
	obj.resources = resource.Drivers{v6, v4}
	v6.SetObject(obj)
	v4.SetObject(obj)
	assert.Equal(t, "ip#0", v6.routeOwner(context.Background()).RID())
	assert.Equal(t, "ip#1", v4.routeOwner(context.Background()).RID())
}

// A resource that cannot start, or that the running action leaves out, is no
// owner: the namespace would be left with no default route.
func TestRouteOwnerStarts(t *testing.T) {
	obj := &fakeObject{}
	own := newRouteTestResource(t, "ip#1", "container#0", gatewayOwn)
	lan := newRouteTestResource(t, "ip#0", "container#0", gatewayOfNetwork)
	obj.resources = resource.Drivers{own, lan}
	own.SetObject(obj)
	lan.SetObject(obj)
	ctx := context.Background()
	assert.Equal(t, "ip#1", lan.routeOwner(ctx).RID())

	selected := actioncontext.WithSelectedRIDs(ctx, obj.Path(), []string{"ip#0"})
	assert.Equal(t, "ip#0", lan.routeOwner(selected).RID(), "ip#1 is not started")

	own.netErr = errors.New("misfit")
	assert.Equal(t, "ip#0", lan.routeOwner(ctx).RID(), "ip#1 cannot start")
}
