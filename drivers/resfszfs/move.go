package resfszfs

import (
	"context"
	"strings"

	"github.com/opensvc/om3/v3/util/zfs"
)

// MoveCopiesStorage implements resource.MoveStorageCopier. A dataset is
// mounted on one node at a time, so the files of the disks of a moving
// container are mirrored to the copy PreMove made on the destination.
func (t *T) MoveCopiesStorage() bool {
	return true
}

// PreMove implements resource.PreMover. It copies the dataset to the
// destination and mounts it there, so the files the container opens exist
// when the move mirrors its disks to them.
//
// The copy is sent from the newest snapshot both nodes hold, which the
// previous move or a sync.zfs left, and in full when there is none.
func (t *T) PreMove(ctx context.Context, to string) error {
	m, err := t.mover(to)
	if err != nil {
		return err
	}
	defer m.Client.Close()
	snap, err := m.Send(ctx)
	if err != nil {
		return err
	}
	t.moveSnapshot = snap
	legacy, err := t.isLegacy()
	if err != nil {
		return err
	}
	return m.Remote(ctx, t.remoteMountCmdline(legacy))
}

// PreMoveRollback implements resource.PreMoveRollbacker. The copy is
// unmounted on the destination, the dataset being mounted on this node, and
// kept, as the base of the next move.
func (t *T) PreMoveRollback(ctx context.Context, to string) error {
	m, err := t.mover(to)
	if err != nil {
		return err
	}
	defer m.Client.Close()
	legacy, err := t.isLegacy()
	if err != nil {
		return err
	}
	return m.Remote(ctx, t.remoteUmountCmdline(legacy))
}

// PostMove implements resource.PostMover. The container runs on the
// destination: the move snapshots older than the one sent are destroyed on
// both nodes, and this node unmounts the dataset when the stop goes on.
//
// The move is done whatever the pruning says, so a failure to prune is
// worth a warning, not a failed move.
func (t *T) PostMove(ctx context.Context, to string) error {
	defer func() { t.moveSnapshot = "" }()
	if t.moveSnapshot == "" {
		return nil
	}
	m, err := t.mover(to)
	if err != nil {
		t.Log().Warnf("%s", err)
		return nil
	}
	defer m.Client.Close()
	if err := m.Prune(ctx, t.moveSnapshot); err != nil {
		t.Log().Warnf("%s", err)
	}
	return nil
}

func (t *T) mover(to string) (*zfs.Mover, error) {
	client, err := t.NewSSHClient(to)
	if err != nil {
		return nil, err
	}
	return &zfs.Mover{Name: t.Device, Node: to, Client: client, Log: t.Log()}, nil
}

// remoteMountCmdline mounts the dataset on the destination as Start mounts
// it here: by its mountpoint property, or on the mount point of the resource
// for a legacy one.
func (t *T) remoteMountCmdline(legacy bool) string {
	opts := t.MountOptions
	if legacy {
		l := []string{"mkdir", "-p", shellQuote(t.mountPoint()), "&&", "mount", "-t", "zfs"}
		if opts != "" {
			l = append(l, "-o", shellQuote(opts))
		}
		l = append(l, shellQuote(t.Device), shellQuote(t.mountPoint()))
		return strings.Join(l, " ")
	}
	return "/usr/sbin/zfs mount " + shellQuote(t.Device)
}

func (t *T) remoteUmountCmdline(legacy bool) string {
	if legacy {
		return "umount " + shellQuote(t.mountPoint())
	}
	return "/usr/sbin/zfs umount " + shellQuote(t.Device)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
