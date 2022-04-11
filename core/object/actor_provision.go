package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/resource"
)

// OptsProvision is the options of the Provision object method.
type OptsProvision struct {
	OptsGlobal
	OptsAsync
	OptsLocking
	OptsResourceSelector
	OptTo
	OptForce
	OptLeader
	OptDisableRollback
}

// Provision allocates and starts the local instance of the object
func (t *Base) Provision(options OptsProvision) error {
	ctx := actioncontext.New(options, objectactionprops.Provision)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("provision", false)
	err := t.lockedAction("", options.OptsLocking, "provision", func() error {
		return t.lockedProvision(ctx)
	})
	if err != nil {
		return err
	}
	if options.IsRollbackDisabled() {
		return nil
	}
	return actionrollback.Rollback(ctx)
}

func (t *Base) lockedProvision(ctx context.Context) error {
	if err := t.masterProvision(ctx); err != nil {
		return err
	}
	if err := t.slaveProvision(ctx); err != nil {
		return err
	}
	return nil
}

func (t *Base) masterProvision(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("provision resource")
		leader := actioncontext.IsLeader(ctx)
		return resource.Provision(ctx, r, leader)
	})
}

func (t *Base) slaveProvision(ctx context.Context) error {
	return nil
}
