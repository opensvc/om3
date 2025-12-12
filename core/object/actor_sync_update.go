package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
)

// SyncUpdate does an immediate data synchronization to target nodes.
func (t *actor) SyncUpdate(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.SyncUpdate)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("sync_update", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedSyncUpdate(ctx)
}

func (t *actor) lockedSyncUpdate(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return resource.Update(ctx, r)
	})
}
