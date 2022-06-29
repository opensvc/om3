package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/resource"
)

// OptsStart is the options of the Start object method.
type OptsStart struct {
	OptsLock
	OptsResourceSelector
	OptTo
	OptForce
	OptDisableRollback
	OptDryRun
}

// Start starts the local instance of the object
func (t *Base) Start(options OptsStart) error {
	props := actioncontext.Start
	ctx := context.Background()
	ctx = actioncontext.WithOptions(ctx, options)
	ctx = actioncontext.WithProps(ctx, props)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("start", false)
	unlock, err := t.lockAction(props, options.OptsLock)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedStart(ctx)
}

func (t *Base) lockedStart(ctx context.Context) error {
	if err := t.masterStart(ctx); err != nil {
		return err
	}
	if err := t.slaveStart(ctx); err != nil {
		return err
	}
	return nil
}

func (t *Base) masterStart(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("start resource")
		return resource.Start(ctx, r)
	})
}

func (t *Base) slaveStart(ctx context.Context) error {
	return nil
}
