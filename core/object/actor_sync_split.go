package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
)

// SyncSplit promote the receiving member of disk pair to read-write
func (t *actor) SyncSplit(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.SyncSplit)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("sync_split", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedSyncSplit(ctx)
}

func (t *actor) lockedSyncSplit(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return resource.Split(ctx, r)
	})
}
