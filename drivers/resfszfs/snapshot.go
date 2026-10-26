package resfszfs

import (
	"context"
	"path/filepath"

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
	// The snapshot is read under the mount point the dataset is
	// mounted on, in the root of the container when zoned, checked free
	// of links first.
	mnt, err := t.resolveMountPoint()
	if err != nil {
		return "", nil, false, err
	}
	if err := snap.Snapshot(); err != nil {
		return "", nil, false, err
	}
	dir := filepath.Join(mnt, ".zfs", "snapshot", name)
	return dir, func() error { return snap.Destroy() }, true, nil
}
