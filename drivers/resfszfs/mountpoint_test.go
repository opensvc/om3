package resfszfs

import (
	"context"
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

// A follower of a shared dataset removes the mount point its start created,
// with the parents created with it: the dataset is the leader's to destroy,
// the mount point every node's own.
func TestUnprovisionAsFollowerRemovesTheMountPointCreated(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	mnt := filepath.Join(root, "lxc", "base")
	r := &T{MountPoint: mnt}
	r.Shared = true
	require.NoError(t, r.SetRID("fs#base"))
	r.SetObject(mountPointObject{varDir: t.TempDir()})
	require.NoError(t, r.createMountPoint(ctx))
	assert.DirExists(t, mnt)
	require.NoError(t, r.UnprovisionAsFollower(ctx))
	assert.NoDirExists(t, filepath.Join(root, "lxc"))
	assert.DirExists(t, root)
}
