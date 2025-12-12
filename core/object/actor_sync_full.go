package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
)

// SyncFull does an immediate data synchronization to target nodes.
func (t *actor) SyncFull(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.SyncFull)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("sync_full", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedSyncFull(ctx)
}

func (t *actor) lockedSyncFull(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return resource.Full(ctx, r)
	})
}
