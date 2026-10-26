package resfshost

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/findmnt"
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

type zoneObject struct {
	resources resource.Drivers
	varDir    string
}

func (o zoneObject) Log() *plog.Logger                   { return plog.NewLogger(zerolog.Nop()) }
func (o zoneObject) VarDir() string                      { return o.varDir }
func (o zoneObject) ResourceByID(string) resource.Driver { return nil }
func (o zoneObject) ResourcesByDrivergroups([]driver.Group) resource.Drivers {
	return o.resources
}

func TestZoneOf(t *testing.T) {
	ctx := context.Background()
	c := fakeContainer{T: &T{}, name: "c1", head: "/var/lib/lxc/c1/rootfs"}
	require.NoError(t, c.SetRID("container#1"))
	o := zoneObject{resources: resource.Drivers{c}}

	z, err := ZoneOf(ctx, o, "c1", "/srv/data/")
	require.NoError(t, err)
	assert.Equal(t, "/var/lib/lxc/c1/rootfs/srv/data", z.Path(), "under the root of the container named")

	z, err = ZoneOf(ctx, o, "container#1", "/srv/data")
	require.NoError(t, err)
	assert.Equal(t, "/var/lib/lxc/c1/rootfs/srv/data", z.Path(), "or of the container of that rid")

	z, err = ZoneOf(ctx, o, "c1", "/")
	require.NoError(t, err)
	assert.True(t, z.IsRoot())

	_, err = ZoneOf(ctx, o, "c2", "/srv/data")
	assert.ErrorContains(t, err, "zone c2", "a zone naming no container is an error, not the host path")
}

// A process of the container planting a link on the path of the mount point
// does not lead the agent out of the root of the container: the check
// refuses it, nothing is made out of it, and the root is never removed.
func TestZoneStaysInTheContainer(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "rootfs")
	host := filepath.Join(base, "host")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "srv"), 0755))
	require.NoError(t, os.MkdirAll(host, 0755))

	z := Zone{Root: root, Rel: "/srv/data"}
	require.NoError(t, z.Check())
	require.NoError(t, z.MkdirAll(0755))
	assert.DirExists(t, filepath.Join(root, "srv", "data"))
	f, err := z.Open()
	require.NoError(t, err)
	_ = f.Close()
	require.NoError(t, z.Remove())
	assert.NoDirExists(t, filepath.Join(root, "srv", "data"))

	require.NoError(t, os.Symlink(host, filepath.Join(root, "etc")))
	z = Zone{Root: root, Rel: "/etc/data"}
	assert.ErrorContains(t, z.Check(), "is a link")
	assert.Error(t, z.MkdirAll(0755))
	assert.NoDirExists(t, filepath.Join(host, "data"), "nothing made out of the container")
	_, err = z.Open()
	assert.Error(t, err)

	file := Zone{Root: root, Rel: "/srv/file"}
	require.NoError(t, file.CreateFile(0755))
	assert.FileExists(t, filepath.Join(root, "srv", "file"))

	assert.Error(t, Zone{Root: root, Rel: "/"}.Remove(), "the root of the container is never removed")
	assert.DirExists(t, root)
}

// A zoned tmpfs mounts in the container, through the mount point opened,
// unmounts the same way, and refuses to start when a link was planted on
// its path.
func TestAZonedFilesystemMountsInTheContainer(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	base := t.TempDir()
	root := filepath.Join(base, "rootfs")
	require.NoError(t, os.MkdirAll(root, 0755))
	c := fakeContainer{T: &T{}, name: "c1", head: root}
	require.NoError(t, c.SetRID("container#1"))

	mnt := filepath.Join(root, "srv", "data")
	t.Cleanup(func() {
		// Never leave a mount behind a failed assertion.
		if mounted, _ := findmnt.HasMnt(context.Background(), mnt); mounted {
			_ = exec.Command("umount", mnt).Run()
		}
	})
	r := &T{Type: "tmpfs", Device: "tmpfs", MountPoint: "/srv/data", Zone: "c1"}
	require.NoError(t, r.SetRID("fs#1"))
	r.SetObject(zoneObject{resources: resource.Drivers{c}, varDir: t.TempDir()})
	ctx := actionrollback.NewContext(context.Background())
	require.NoError(t, r.mount(ctx))
	mounted, err := findmnt.HasMnt(ctx, filepath.Join(root, "srv", "data"))
	require.NoError(t, err)
	assert.True(t, mounted, "mounted at the path in the container")
	require.NoError(t, r.Stop(ctx))
	mounted, err = findmnt.HasMnt(ctx, filepath.Join(root, "srv", "data"))
	require.NoError(t, err)
	assert.False(t, mounted, "unmounted")

	host := filepath.Join(base, "host")
	require.NoError(t, os.MkdirAll(host, 0755))
	require.NoError(t, os.RemoveAll(filepath.Join(root, "srv")))
	require.NoError(t, os.Symlink(host, filepath.Join(root, "srv")))
	_, err = r.isMounted(ctx)
	assert.ErrorContains(t, err, "is a link", "the mounted check refuses the path too, whatever findmnt says of an empty one")
	assert.ErrorContains(t, r.mount(ctx), "is a link")
	assert.NoDirExists(t, filepath.Join(host, "data"), "nothing made nor mounted out of the container")
}

func TestValidateMountPoint(t *testing.T) {
	assert.NoError(t, validateMountPoint("/srv/data"))
	assert.ErrorContains(t, validateMountPoint("relmnt-data"), "not an absolute path")
	assert.ErrorContains(t, validateMountPoint("./data"), "not an absolute path")
}

// The file of a zoned file bind mount is the one checked: a link swapped in
// at its name, even one staying in the container, is not followed.
func TestZoneOpenFileRefusesALink(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "srv"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "srv", "other"), nil, 0644))
	z := Zone{Root: root, Rel: "/srv/file"}
	require.NoError(t, z.CreateFile(0755))
	f, err := z.Open()
	require.NoError(t, err)
	_ = f.Close()

	require.NoError(t, os.Remove(filepath.Join(root, "srv", "file")))
	require.NoError(t, os.Symlink("other", filepath.Join(root, "srv", "file")))
	_, err = z.Open()
	assert.Error(t, err, "a link at the name of the file is refused")
}
