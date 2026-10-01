package resdiskzvol

import (
	"context"

	"github.com/opensvc/om3/v3/util/zfs"
)

// MoveCopiesStorage implements resource.MoveStorageCopier. A zvol is in a
// pool imported on one node at a time, so the device of a moving container
// is mirrored to the copy PreMove made on the destination.
func (t *T) MoveCopiesStorage() bool {
	return true
}

// PreMove implements resource.PreMover. It copies the zvol to the
// destination, so the device the container opens exists there when the move
// mirrors its disk to it.
//
// The copy is sent from the newest snapshot both nodes hold, which the
// previous move left, and in full when there is none. The device node is
// created by udev once the receive is done, which the destination waits
// for.
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
	return m.Remote(ctx, "udevadm settle && test -e "+t.devpath())
}

// PostMove implements resource.PostMover. The container runs on the
// destination: the move snapshots older than the one sent are destroyed on
// both nodes.
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
	return &zfs.Mover{Name: t.Name, Node: to, Client: client, Log: t.Log()}, nil
}
