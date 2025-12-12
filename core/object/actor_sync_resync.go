package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
)

// SyncResync re-establishes the data synchronization
func (t *actor) SyncResync(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.SyncResync)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("sync_resync", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedSyncResync(ctx)
}

func (t *actor) lockedSyncResync(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return resource.Resync(ctx, r)
	})
}
