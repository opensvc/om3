package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
)

func (t *actor) SyncRestore(ctx context.Context, to, src string) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.SyncRestore)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("sync_restore", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedSyncRestore(ctx, to, src)
}

func (t *actor) lockedSyncRestore(ctx context.Context, to, src string) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return resource.Restore(ctx, r, to, src)
	})
}
