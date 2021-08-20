package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resourceselector"
)

// OptsSyncResync is the options of the SyncResync object method.
type OptsSyncResync struct {
	OptsGlobal
	OptsLocking
	resourceselector.Options
	OptForce
}

// SyncResync re-establishes the data synchronization
func (t *Base) SyncResync(options OptsSyncResync) error {
	ctx := actioncontext.New(options, objectactionprops.SyncResync)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("sync_resync", false)
	defer t.postActionStatusEval(ctx)
	return t.lockedAction("", options.OptsLocking, "sync_resync", func() error {
		return t.lockedSyncResync(ctx)
	})

}

func (t *Base) lockedSyncResync(ctx context.Context) error {
	if err := t.masterSyncResync(ctx); err != nil {
		return err
	}
	if err := t.slaveSyncResync(ctx); err != nil {
		return err
	}
	return nil
}

func (t *Base) masterSyncResync(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return resource.Resync(ctx, r)
	})
}

func (t *Base) slaveSyncResync(ctx context.Context) error {
	return nil
}
