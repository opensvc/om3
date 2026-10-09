package fssnap

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/util/lvm2"
)

const (
	// minLVSnapSize is the least copy-on-write space a thick snapshot is
	// given, for a small volume written to while it is copied.
	minLVSnapSize = 32 * 1024 * 1024

	// lvRemoveAttempts is how many times a snapshot is asked removed, as
	// udev can keep it busy a moment after it was unmounted.
	lvRemoveAttempts = 10
)

// snapshotLV snapshots the logical volume lv, and mounts the snapshot
// read-only. A thick volume is given a tenth of its size, as v2 did, to hold
// what is written to the volume while the snapshot is read: lvm drops a
// snapshot running out of it, and the copy then fails rather than reading a
// torn source.
func (t *Set) snapshotLV(ctx context.Context, h *holder, i int, lv *lvm2.LV) error {
	name := t.Name + "_" + lv.LVName
	if len(name) > 127 {
		return fmt.Errorf("the snapshot name %s is longer than lvm allows", name)
	}
	snap := lvm2.NewLV(lv.VGName, name, lvm2.WithLogger(t.Log))
	if err := t.record(entry{Kind: kindLV, Path: snap.FQN()}); err != nil {
		return err
	}
	size, err := lv.Size(ctx)
	if err != nil {
		return err
	}
	if _, err := lv.CreateSnapshot(ctx, name, max(size/10, minLVSnapSize)); err != nil {
		return err
	}
	opts := "ro"
	if h.mount.FSType == "xfs" {
		// The snapshot has the uuid of the filesystem it is of, which is
		// mounted.
		opts += ",nouuid"
	}
	p, err := t.mount(ctx, i, h.mount.FSType, opts, snap.DevPath())
	if err != nil {
		return err
	}
	// A mount of a directory of the filesystem, as a bind mount is, is
	// that directory of the snapshot.
	h.root = filepath.Join(p, h.mount.Root)
	return nil
}

// undoLV removes the snapshot fqn, <vg>/<lv>, when it exists.
func (t *Set) undoLV(ctx context.Context, fqn string) error {
	vg, name, ok := strings.Cut(fqn, "/")
	if !ok {
		return fmt.Errorf("journal of %s: %s is no <vg>/<lv>", t.Dir, fqn)
	}
	snap := lvm2.NewLV(vg, name, lvm2.WithLogger(t.Log))
	if v, err := snap.Exists(ctx); err != nil {
		return err
	} else if !v {
		// Never made, or already removed.
		return nil
	}
	var err error
	for attempt := 1; attempt <= lvRemoveAttempts; attempt++ {
		if err = snap.Remove(ctx, []string{"-f"}); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(time.Second):
		}
	}
	return err
}
