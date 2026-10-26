package resfszfs

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/drivers/resfshost"
	"github.com/opensvc/om3/v3/util/zfs"
)

// SnapshotHead implements resource.HeadSnapshotter: a snapshot of the
// dataset, read in the .zfs/snapshot directory of its mount point, and
// destroyed at the release.
func (t *T) SnapshotHead(ctx context.Context, name string) (string, func() error, bool, error) {
	snap := &zfs.Filesystem{
		Log:  t.Log().Attr("device", t.Device).WithPrefix(t.Log().Prefix() + t.Device + ": "),
		Name: t.Device + "@" + name,
	}
	z, err := t.zoneOf()
	if err != nil {
		return "", nil, false, err
	}
	if z == nil {
		if err := snap.Snapshot(); err != nil {
			return "", nil, false, err
		}
		dir := filepath.Join(t.mountPoint(), ".zfs", "snapshot", name)
		return dir, func() error { return snap.Destroy() }, true, nil
	}
	// A zoned dataset is mounted under the root of a container, whose
	// processes could lead its path elsewhere before the store opens it:
	// the snapshot is opened through the root of the dataset, itself
	// opened through the root of the container, and the store is handed
	// the descriptor, open until the release. The .zfs directory is the
	// dataset's own, which no process of the container replaces.
	root, err := z.Open()
	if err != nil {
		return "", nil, false, err
	}
	if err := snap.Snapshot(); err != nil {
		_ = root.Close()
		return "", nil, false, err
	}
	release := func(dir *os.File) func() error {
		return func() error {
			var errs error
			if dir != nil {
				errs = dir.Close()
			}
			_ = root.Close()
			return errors.Join(errs, snap.Destroy())
		}
	}
	r, err := os.OpenRoot(resfshost.FDPath(root))
	if err != nil {
		return "", nil, false, errors.Join(err, release(nil)())
	}
	defer r.Close()
	dir, err := r.Open(filepath.Join(".zfs", "snapshot", name))
	if err != nil {
		return "", nil, false, errors.Join(err, release(nil)())
	}
	// The trailing slash makes a path walk go through the descriptor to
	// the directory, rather than stop at /proc/<pid>/fd/<fd> as a link.
	return resfshost.FDPath(dir) + "/", release(dir), true, nil
}
