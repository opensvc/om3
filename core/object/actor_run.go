package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/resource"
)

// OptsRun is the options of the Run object method.
type OptsRun struct {
	OptsGlobal
	OptsLocking
	OptsResourceSelector
	OptConfirm
}

// Run starts the local instance of the object
func (t *Base) Run(options OptsRun) error {
	ctx := actioncontext.New(options, objectactionprops.Run)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("run", false)
	if err := t.masterRun(ctx); err != nil {
		return err
	}
	if err := t.slaveRun(ctx); err != nil {
		return err
	}
	return nil
}

func (t *Base) masterRun(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("run resource")
		return resource.Run(ctx, r)
	})
}

func (t *Base) slaveRun(ctx context.Context) error {
	return nil
}
