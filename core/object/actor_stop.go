package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/resource"
)

// OptsStop is the options of the Stop object method.
type OptsStop struct {
	OptsGlobal
	OptsAsync
	OptsLocking
	OptsResourceSelector
	OptTo
	OptForce
}

// Stop stops the local instance of the object
func (t *Base) Stop(options OptsStop) error {
	ctx := actioncontext.New(options, objectactionprops.Stop)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("stop", false)
	return t.lockedAction("", options.OptsLocking, "stop", func() error {
		return t.lockedStop(ctx)
	})

}

func (t *Base) lockedStop(ctx context.Context) error {
	if err := t.masterStop(ctx); err != nil {
		return err
	}
	if err := t.slaveStop(ctx); err != nil {
		return err
	}
	return nil
}

func (t *Base) masterStop(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("stop resource")
		return resource.Stop(ctx, r)
	})
}

func (t *Base) slaveStop(ctx context.Context) error {
	return nil
}
