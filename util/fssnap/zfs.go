package fssnap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/util/zfs"
)

// snapshotZFS snapshots the dataset mounted, whose snapshot is read in the
// .zfs directory of its mount point, whatever its snapdir property.
func (t *Set) snapshotZFS(_ context.Context, h *holder) error {
	snap := zfs.Filesystem{Name: h.mount.Source + "@" + t.Name, Log: t.Log}
	if err := t.record(entry{Kind: kindZFS, Path: snap.Name}); err != nil {
		return err
	}
	if err := snap.Snapshot(); err != nil {
		return err
	}
	// Reading it mounts it.
	root := filepath.Join(h.mount.Target, ".zfs", "snapshot", t.Name)
	if _, err := os.Stat(root); err != nil {
		return fmt.Errorf("the snapshot %s is not reached at %s: %w", snap.Name, root, err)
	}
	h.root = root
	return nil
}

// undoZFS destroys the snapshot name, when it exists, which unmounts it.
func (t *Set) undoZFS(_ context.Context, name string) error {
	snap := zfs.Filesystem{Name: name, Log: t.Log}
	if v, err := snap.SnapshotExists(); err != nil {
		return err
	} else if !v {
		// Never made, or already destroyed.
		return nil
	}
	return snap.Destroy()
}
