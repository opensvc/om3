package resfshost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/lvm2"
)

// SnapshotHead implements resource.HeadSnapshotter: a read-only snapshot of a
// btrfs subvolume mounted as the head, or a snapshot of an LVM thin volume
// holding the filesystem, mounted in a private directory. Other storage
// takes none.
func (t *T) SnapshotHead(ctx context.Context, name string) (string, func() error, bool, error) {
	if t.Type == "btrfs" {
		return t.snapshotBtrfs(ctx, name)
	}
	dev := t.devpath(ctx)
	if !strings.HasPrefix(dev, "/dev/") {
		return "", nil, false, nil
	}
	lv := t.thinLV(ctx, dev)
	if lv == nil {
		return "", nil, false, nil
	}
	return t.snapshotThinLV(ctx, lv, name)
}

// snapshotBtrfs snapshots the subvolume mounted as the head into a hidden
// directory of it, which the snapshot does not hold, and deletes it at the
// release. A head that is not the root of a subvolume takes none.
func (t *T) snapshotBtrfs(ctx context.Context, name string) (string, func() error, bool, error) {
	mnt := t.mountPoint()
	if err := t.run(ctx, false, "btrfs", "subvolume", "show", mnt); err != nil {
		return "", nil, false, nil
	}
	dir := filepath.Join(mnt, "."+name)
	if err := t.run(ctx, true, "btrfs", "subvolume", "snapshot", "-r", mnt, dir); err != nil {
		return "", nil, false, err
	}
	return dir, func() error {
		return t.run(context.Background(), true, "btrfs", "subvolume", "delete", dir)
	}, true, nil
}

// thinLV returns the LVM thin volume dev is, and nil when it is none.
func (t *T) thinLV(ctx context.Context, dev string) *lvm2.LV {
	lv, err := lvm2.LVOfDevice(ctx, dev, lvm2.WithLogger(t.Log()))
	if err != nil || lv == nil {
		return nil
	}
	info, err := lv.Show(ctx)
	if err != nil || info.SegType != "thin" {
		return nil
	}
	return lv
}

// snapshotThinLV snapshots the thin volume, and mounts the snapshot in a
// private directory: xfs refuses a second mount of the uuid of the volume
// without nouuid. The release unmounts it and removes the snapshot.
func (t *T) snapshotThinLV(ctx context.Context, lv *lvm2.LV, name string) (string, func() error, bool, error) {
	snap, err := lv.CreateSnapshot(ctx, lv.LVName+"-"+name, 0)
	if err != nil {
		return "", nil, false, err
	}
	removeSnap := func() error {
		return snap.Remove(context.Background(), []string{"-f"})
	}
	dir, err := os.MkdirTemp("", "opensvc-snapshot-")
	if err != nil {
		return "", nil, false, errors.Join(err, removeSnap())
	}
	args := []string{"-t", t.Type}
	if t.Type == "xfs" {
		args = append(args, "-o", "nouuid")
	}
	args = append(args, snap.DevPath(), dir)
	if err := t.run(ctx, true, "mount", args...); err != nil {
		return "", nil, false, errors.Join(err, os.Remove(dir), removeSnap())
	}
	return dir, func() error {
		if err := t.run(context.Background(), true, "umount", dir); err != nil {
			return fmt.Errorf("unmount the snapshot %s from %s, kept: %w", snap.FQN(), dir, err)
		}
		return errors.Join(os.Remove(dir), removeSnap())
	}, true, nil
}

// run runs the command name with args, logged at info level when it changes
// something, at debug level when it only asks.
func (t *T) run(ctx context.Context, changes bool, name string, args ...string) error {
	level := zerolog.DebugLevel
	if changes {
		level = zerolog.InfoLevel
	}
	return command.New(
		command.WithContext(ctx),
		command.WithName(name),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(level),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(level),
		command.WithErrorExitCodeLogLevel(level),
	).Run()
}
