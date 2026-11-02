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

// The parents a start created with the mount point are removed with it, each
// empty, from the deepest: one holding something else stops the removal, and
// one that existed before is the operator's.
func TestUnprovisionRemovesTheParentsCreatedWithTheMountPoint(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	mnt := filepath.Join(root, "a", "b", "c")
	r := newMountPointResource(t, mnt)
	require.NoError(t, r.createMountPoint(ctx))
	require.NoError(t, r.UnprovisionAsLeader(ctx))
	assert.NoDirExists(t, filepath.Join(root, "a"), "the 3 levels created are removed")
	assert.DirExists(t, root, "what existed before stays")

	// A parent holding something else stops the removal there.
	r = newMountPointResource(t, mnt)
	require.NoError(t, r.createMountPoint(ctx))
	require.NoError(t, os.WriteFile(filepath.Join(root, "a", "other"), nil, 0o600))
	require.NoError(t, r.UnprovisionAsFollower(ctx))
	assert.NoDirExists(t, filepath.Join(root, "a", "b"))
	assert.DirExists(t, filepath.Join(root, "a"), "holding another file")
	require.NoError(t, os.RemoveAll(filepath.Join(root, "a")))

	// A parent that existed before is not one the start created.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "a"), 0o755))
	r = newMountPointResource(t, mnt)
	require.NoError(t, r.createMountPoint(ctx))
	require.NoError(t, r.UnprovisionAsLeader(ctx))
	assert.NoDirExists(t, filepath.Join(root, "a", "b"))
	assert.DirExists(t, filepath.Join(root, "a"), "found by the start")
	require.NoError(t, os.RemoveAll(filepath.Join(root, "a")))

	// A parent replaced since is the operator's, and so is what is above.
	r = newMountPointResource(t, mnt)
	require.NoError(t, r.createMountPoint(ctx))
	require.NoError(t, os.Remove(mnt))
	// Made while the former exists, for the inode not to be reused.
	require.NoError(t, os.Mkdir(filepath.Join(root, "a", "b2"), 0o755))
	require.NoError(t, os.Remove(filepath.Join(root, "a", "b")))
	require.NoError(t, os.Rename(filepath.Join(root, "a", "b2"), filepath.Join(root, "a", "b")))
	require.NoError(t, r.UnprovisionAsLeader(ctx))
	assert.DirExists(t, filepath.Join(root, "a", "b"), "replaced")
	assert.DirExists(t, filepath.Join(root, "a"))
	assert.NoFileExists(t, r.mountPointCreatedFile(), "forgotten")
}

// The mount point recorded by an agent knowing no parent is removed, and its
// parents left.
func TestUnprovisionReadsARecordWithNoParent(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	mnt := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(mnt, 0o755))
	r := newMountPointResource(t, mnt)
	id, err := mountPointIdentity(mnt)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(r.mountPointCreatedFile()), 0o700))
	require.NoError(t, os.WriteFile(r.mountPointCreatedFile(), []byte(mnt+"\n"+id+"\n"), 0o600))
	require.NoError(t, r.UnprovisionAsLeader(ctx))
	assert.NoDirExists(t, mnt)
	assert.DirExists(t, filepath.Join(root, "a"))
}

// In a zone, the parents created in the container are removed through its
// root, and the root of the container is never removed.
func TestZoneRemovesTheParentsCreatedWithTheMountPoint(t *testing.T) {
	root := t.TempDir()
	z := Zone{Root: root, Rel: "/srv/x/y"}
	parents := MissingParentsInZone(z)
	assert.Equal(t, []string{filepath.Join(root, "srv", "x"), filepath.Join(root, "srv")}, parents)
	require.NoError(t, z.MkdirAll(0o755))
	rec := MountPointRecord{File: filepath.Join(t.TempDir(), "mnt_created"), Log: plog.NewLogger(zerolog.Nop())}
	rec.MarkInZone(z, parents)

	// A link the container made of a parent leads nowhere out of it.
	outside := t.TempDir()
	require.NoError(t, os.Remove(filepath.Join(root, "srv", "x", "y")))
	require.NoError(t, os.Remove(filepath.Join(root, "srv", "x")))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "srv", "x")))
	rec.RemoveInZone(z)
	assert.DirExists(t, outside)
	assert.FileExists(t, filepath.Join(root, "srv", "x"), "the link is not the directory created")
	require.NoError(t, os.Remove(filepath.Join(root, "srv", "x")))

	require.NoError(t, z.MkdirAll(0o755))
	rec.MarkInZone(z, MissingParentsInZone(Zone{Root: root, Rel: "/srv/x/y"}))
	rec.MarkInZone(z, []string{filepath.Join(root, "srv", "x")})
	rec.RemoveInZone(z)
	assert.NoDirExists(t, filepath.Join(root, "srv", "x"))
	assert.DirExists(t, root)
}
