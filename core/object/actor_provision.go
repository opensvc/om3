package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/resource"
)

// OptsProvision is the options of the Provision object method.
type OptsProvision struct {
	OptsLock
	OptsResourceSelector
	OptDryRun
	OptTo
	OptForce
	OptLeader
	OptDisableRollback
}

// Provision allocates and starts the local instance of the object
func (t *core) Provision(options OptsProvision) error {
	props := actioncontext.Provision
	ctx := context.Background()
	ctx = actioncontext.WithOptions(ctx, options)
	ctx = actioncontext.WithProps(ctx, props)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("provision", false)
	unlock, err := t.lockAction(props, options.OptsLock)
	if err != nil {
		return err
	}
	defer unlock()
	if err := t.lockedProvision(ctx); err != nil {
		return err
	}
	if options.IsRollbackDisabled() {
		// --disable-rollback handling
		return nil
	}
	return t.lockedStop(ctx)
}

func (t *core) lockedProvision(ctx context.Context) error {
	if err := t.masterProvision(ctx); err != nil {
		return err
	}
	if err := t.slaveProvision(ctx); err != nil {
		return err
	}
	return nil
}

func (t *core) masterProvision(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("provision resource")
		leader := actioncontext.IsLeader(ctx)
		return resource.Provision(ctx, r, leader)
	})
}

func (t *core) slaveProvision(ctx context.Context) error {
	return nil
}
