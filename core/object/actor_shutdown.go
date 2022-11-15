package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/resource"
)

// Shutdown shuts down the local instance of the object
func (t *actor) Shutdown(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Shutdown)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("shutdown", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedShutdown(ctx)
}

func (t *actor) lockedShutdown(ctx context.Context) error {
	if err := t.masterShutdown(ctx); err != nil {
		return err
	}
	if err := t.slaveShutdown(ctx); err != nil {
		return err
	}
	return nil
}

func (t *actor) masterShutdown(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("shutdown resource")
		return resource.Shutdown(ctx, r)
	})
}

func (t *actor) slaveShutdown(ctx context.Context) error {
	return nil
}
