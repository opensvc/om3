package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/resource"
)

// OptsUnprovision is the options of the Unprovision object method.
type OptsUnprovision struct {
	OptsLock
	OptsResourceSelector
	OptTo
	OptForce
	OptLeader
	OptDryRun
}

// Unprovision stops and frees the local instance of the object
func (t *core) Unprovision(options OptsUnprovision) error {
	props := actioncontext.Unprovision
	ctx := context.Background()
	ctx = actioncontext.WithOptions(ctx, options)
	ctx = actioncontext.WithProps(ctx, props)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("unprovision", false)
	unlock, err := t.lockAction(props, options.OptsLock)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedUnprovision(ctx)
}

func (t *core) lockedUnprovision(ctx context.Context) error {
	if err := t.slaveUnprovision(ctx); err != nil {
		return err
	}
	if err := t.masterUnprovision(ctx); err != nil {
		return err
	}
	return nil
}

func (t *core) masterUnprovision(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("unprovision resource")
		leader := actioncontext.IsLeader(ctx)
		return resource.Unprovision(ctx, r, leader)
	})
}

func (t *core) slaveUnprovision(ctx context.Context) error {
	return nil
}
