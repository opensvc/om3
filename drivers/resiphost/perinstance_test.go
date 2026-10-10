package resiphost

import (
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
)

// fakeObject is the object of the resource, which says its topology.
type fakeObject struct {
	topology topology.T
}

func (t fakeObject) Log() *plog.Logger                   { return plog.NewLogger(zerolog.New(io.Discard)) }
func (t fakeObject) VarDir() string                      { return "" }
func (t fakeObject) ResourceByID(string) resource.Driver { return nil }
func (t fakeObject) Topology() topology.T                { return t.topology }
func (t fakeObject) ResourcesByDrivergroups([]driver.Group) resource.Drivers {
	return nil
}

func newPerInstanceTestResource(t *testing.T, topo topology.T, shared bool) *T {
	t.Helper()
	r := &T{Network: "san"}
	rid, err := resourceid.Parse("ip#0")
	require.NoError(t, err)
	r.ResourceID = rid
	r.Path = naming.Path{Kind: naming.KindSvc, Name: "web"}
	r.Shared = shared
	r.SetObject(fakeObject{topology: topo})
	return r
}

// The instances of a flex resource not shared run at once, and each draws an
// address of its own, recorded under its node. A failover resource, or a
// shared flex one, draws one address for all its instances.
func TestPerInstance(t *testing.T) {
	node := hostname.Hostname()

	r := newPerInstanceTestResource(t, topology.Flex, false)
	assert.True(t, r.perInstance())
	assert.Equal(t, "ip#0@"+node, r.alloc().RID, "the reservation of this instance")
	assert.Equal(t, "addr@"+node, r.addrKey().Option)

	for _, r := range []*T{
		newPerInstanceTestResource(t, topology.Flex, true),
		newPerInstanceTestResource(t, topology.Failover, false),
	} {
		assert.False(t, r.perInstance())
		assert.Equal(t, "ip#0", r.alloc().RID)
		assert.Equal(t, "addr", r.addrKey().Option)
	}
}
