package resfshost

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/plog"
)

// fakeContainer is a container resource holding a root directory, as the
// container.lxc one, a filesystem resource standing for the driver.
type fakeContainer struct {
	*T
	name string
	head string
}

func (t fakeContainer) ContainerName() string { return t.name }
func (t fakeContainer) ContainerHead(context.Context) (string, error) {
	return t.head, nil
}

type zoneObject struct{ resources resource.Drivers }

func (o zoneObject) Log() *plog.Logger                   { return plog.NewLogger(zerolog.Nop()) }
func (o zoneObject) VarDir() string                      { return "" }
func (o zoneObject) ResourceByID(string) resource.Driver { return nil }
func (o zoneObject) ResourcesByDrivergroups([]driver.Group) resource.Drivers {
	return o.resources
}

func TestZoneMountPoint(t *testing.T) {
	ctx := context.Background()
	c := fakeContainer{T: &T{}, name: "c1", head: "/var/lib/lxc/c1/rootfs"}
	require.NoError(t, c.SetRID("container#1"))
	o := zoneObject{resources: resource.Drivers{c}}

	p, err := ZoneMountPoint(ctx, o, "", "/srv/data/")
	require.NoError(t, err)
	assert.Equal(t, "/srv/data", p, "no zone, the host path")

	p, err = ZoneMountPoint(ctx, o, "c1", "/srv/data")
	require.NoError(t, err)
	assert.Equal(t, "/var/lib/lxc/c1/rootfs/srv/data", p, "under the root of the container named")

	p, err = ZoneMountPoint(ctx, o, "container#1", "/srv/data")
	require.NoError(t, err)
	assert.Equal(t, "/var/lib/lxc/c1/rootfs/srv/data", p, "or of the container of that rid")

	_, err = ZoneMountPoint(ctx, o, "c2", "/srv/data")
	assert.ErrorContains(t, err, "zone c2", "a zone naming no container is an error, not the host path")
}

// The resource mounts in the container the zone names, and refuses to act
// when it names none, rather than mount on the host path.
func TestAFilesystemOfAZoneMountsInTheContainer(t *testing.T) {
	c := fakeContainer{T: &T{}, name: "c1", head: "/var/lib/lxc/c1/rootfs"}
	require.NoError(t, c.SetRID("container#1"))
	r := &T{Type: "tmpfs", MountPoint: "/srv/data", Zone: "c1"}
	require.NoError(t, r.SetRID("fs#1"))
	r.SetObject(zoneObject{resources: resource.Drivers{c}})
	assert.Equal(t, "/var/lib/lxc/c1/rootfs/srv/data", r.mountPoint())
	assert.Equal(t, "/var/lib/lxc/c1/rootfs/srv/data", r.Head())

	r = &T{Type: "tmpfs", MountPoint: "/srv/data", Zone: "c2"}
	require.NoError(t, r.SetRID("fs#1"))
	r.SetObject(zoneObject{})
	assert.ErrorContains(t, r.Start(context.Background()), "zone c2")
}

func TestValidateMountPoint(t *testing.T) {
	assert.NoError(t, validateMountPoint("/srv/data"))
	assert.ErrorContains(t, validateMountPoint("relmnt-data"), "not an absolute path")
	assert.ErrorContains(t, validateMountPoint("./data"), "not an absolute path")
}
