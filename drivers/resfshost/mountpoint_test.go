package resfshost

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/plog"
)

type mountPointObject struct{ varDir string }

func (o mountPointObject) Log() *plog.Logger                   { return plog.NewLogger(zerolog.Nop()) }
func (o mountPointObject) VarDir() string                      { return o.varDir }
func (o mountPointObject) ResourceByID(string) resource.Driver { return nil }
func (o mountPointObject) ResourcesByDrivergroups([]driver.Group) resource.Drivers {
	return nil
}

func newMountPointResource(t *testing.T, mnt string) *T {
	t.Helper()
	r := &T{Type: "tmpfs", MountPoint: mnt}
	require.NoError(t, r.SetRID("fs#1"))
	r.SetObject(mountPointObject{varDir: t.TempDir()})
	return r
}

// The unprovision removes the mount point a start of the resource created,
// once empty, and leaves the one the start found in place, which is the
// operator's, and one still holding something.
func TestUnprovisionRemovesTheMountPointItCreated(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	created := filepath.Join(root, "created")
	r := newMountPointResource(t, created)
	require.NoError(t, r.createMountPoint(ctx))
	require.NoError(t, r.UnprovisionAsLeader(ctx))
	assert.NoDirExists(t, created, "the mount point the start created is removed")
	assert.NoFileExists(t, r.mountPointCreatedFile(), "and forgotten")

	found := filepath.Join(root, "found")
	require.NoError(t, os.Mkdir(found, 0755))
	r = newMountPointResource(t, found)
	require.NoError(t, r.createMountPoint(ctx))
	require.NoError(t, r.UnprovisionAsFollower(ctx))
	assert.DirExists(t, found, "the mount point the start found is the operator's")

	full := filepath.Join(root, "full")
	r = newMountPointResource(t, full)
	require.NoError(t, r.createMountPoint(ctx))
	require.NoError(t, os.WriteFile(filepath.Join(full, "data"), nil, 0600))
	require.NoError(t, r.UnprovisionAsLeader(ctx))
	assert.FileExists(t, filepath.Join(full, "data"), "what the mount point holds is left")

	replaced := filepath.Join(root, "replaced")
	r = newMountPointResource(t, replaced)
	require.NoError(t, r.createMountPoint(ctx))
	require.NoError(t, os.Remove(replaced))
	require.NoError(t, os.WriteFile(filepath.Join(root, "hold-the-inode"), nil, 0600))
	require.NoError(t, os.Mkdir(replaced, 0755))
	require.NoError(t, r.UnprovisionAsLeader(ctx))
	assert.DirExists(t, replaced, "a mount point replaced since the start created it is the operator's")
	assert.NoFileExists(t, r.mountPointCreatedFile(), "and the record of the one created is forgotten")

	r.MountPoint = filepath.Join(root, "other")
	require.NoError(t, os.Mkdir(r.MountPoint, 0755))
	require.NoError(t, r.UnprovisionAsLeader(ctx))
	assert.DirExists(t, r.MountPoint, "a mnt changed since is not the one the start created")
}
