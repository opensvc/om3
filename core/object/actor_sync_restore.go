package object

import (
	"context"
	"fmt"
	"slices"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
)

// SyncRestore writes back the data the sync resources backed up, from a
// snapshot, as actioncontext.Restore says.
func (t *actor) SyncRestore(ctx context.Context) error {
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
	return t.lockedSyncRestore(ctx)
}

func (t *actor) lockedSyncRestore(ctx context.Context) error {
	if opts := actioncontext.Restore(ctx); opts.Stop {
		if opts.Into != "" || opts.Config {
			return fmt.Errorf("--stop readies the instance for a restore of the heads in place, which --into and --config are not")
		}
		if err := t.lockedRestoreReady(ctx); err != nil {
			return err
		}
	}
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return resource.Restore(ctx, r)
	})
}

// lockedRestoreReady brings the instance to the state a restore in place
// needs: the resources using the heads stopped, then the heads started. The
// instance is flagged stopped first, as a stop of the instance flags it, so
// the daemon starts nothing back during the restore, and left so: the
// operator starts the instance once the data restored is checked.
//
// The steps act on every resource of their groups, whatever the restore
// selects, which is its sync resources.
func (t *actor) lockedRestoreReady(ctx context.Context) error {
	if err := t.SetStopped(); err != nil {
		return err
	}
	ctx = actioncontext.WithoutResourceSelector(ctx)
	stopCtx := actioncontext.WithProps(ctx, actioncontext.RestoreStop)
	err := t.action(stopCtx, func(ctx context.Context, r resource.Driver) error {
		if !slices.Contains(restoreConsumerGroups, r.ID().DriverGroup()) {
			return nil
		}
		return resource.Stop(ctx, r)
	})
	if err != nil {
		return err
	}
	startCtx := actioncontext.WithProps(ctx, actioncontext.RestoreStart)
	return t.action(startCtx, func(ctx context.Context, r resource.Driver) error {
		if !slices.Contains(restoreHeadGroups, r.ID().DriverGroup()) {
			return nil
		}
		return resource.Start(ctx, r)
	})
}

var (
	// restoreConsumerGroups are the driver groups of the resources
	// using the heads, which a restore in place needs stopped.
	restoreConsumerGroups = []driver.Group{driver.GroupApp, driver.GroupContainer, driver.GroupShare}

	// restoreHeadGroups are the driver groups of the resources the heads
	// are made of, which a restore in place needs started.
	restoreHeadGroups = []driver.Group{driver.GroupVolume, driver.GroupDisk, driver.GroupFS}
)
