package resfsdir

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// fakeVolume is a volume resource with a mount point and a status, what
	// the path of a directory in it is resolved from.
	fakeVolume struct {
		resource.Driver
		head  string
		avail status.T
	}

	// fakeObject is the object of the directory, holding its volume.
	fakeObject struct {
		resources map[string]resource.Driver
	}
)

func (t fakeVolume) Head() string                    { return t.head }
func (t fakeVolume) Status(context.Context) status.T { return t.avail }

func (t fakeObject) Log() *plog.Logger                       { return plog.NewLogger(zerolog.New(io.Discard)) }
func (t fakeObject) VarDir() string                          { return "" }
func (t fakeObject) ResourceByID(rid string) resource.Driver { return t.resources[rid] }
func (t fakeObject) ResourcesByDrivergroups([]driver.Group) resource.Drivers {
	return nil
}

func TestDirPath(t *testing.T) {
	ctx := context.Background()
	o := &T{DirPath: "/srv/www"}
	p, err := o.dirPath(ctx, true)
	require.NoError(t, err)
	assert.Equal(t, "/srv/www", p)
	assert.Equal(t, "/srv/www", o.Head())

	// A directory is a resource paths are written in, so one written in
	// another resource could name a directory naming it back.
	o = &T{DirPath: "fs#2:/www"}
	_, err = o.dirPath(ctx, false)
	assert.ErrorContains(t, err, "written volume#<n>:/<path>")
	assert.Empty(t, o.Head())

	_, err = (&T{}).dirPath(ctx, false)
	assert.Error(t, err)
}

// TestUnprovisionLeavesTheDirectoryOfAVolumeNotAvailable pins that a
// directory in a volume that is not available is not removed: the path it
// would have is where the volume mounts, and what is there meanwhile is not
// the directory.
func TestUnprovisionLeavesTheDirectoryOfAVolumeNotAvailable(t *testing.T) {
	ctx := context.Background()
	head := t.TempDir()
	dir := filepath.Join(head, "www")
	vol := &fakeVolume{head: head, avail: status.Down}
	o := &T{DirPath: "volume#1:/www"}
	rid, err := resourceid.Parse("fs#1")
	require.NoError(t, err)
	o.ResourceID = rid
	o.SetObject(fakeObject{resources: map[string]resource.Driver{"volume#1": vol}})

	require.NoError(t, os.Mkdir(dir, 0o755))
	require.NoError(t, o.Unprovision(ctx))
	require.NoError(t, o.UnprovisionAsLeader(ctx))
	assert.DirExists(t, dir, "what the mount point hides is not the directory")

	vol.avail = status.Up
	require.NoError(t, o.UnprovisionAsLeader(ctx))
	assert.NoDirExists(t, dir)
}
