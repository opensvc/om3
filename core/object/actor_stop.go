package object

import (
	"context"

	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/resource"
)

// OptsStop is the options of the Stop object method.
type OptsStop struct {
	OptsGlobal
	OptsAsync
	OptsLocking
	OptsResourceSelector
	OptForce
}

// Stop stops the local instance of the object
func (t *Base) Stop(options OptsStop) error {
	defer t.setActionOptions(options)()
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("stop", false)
	defer t.postActionStatusEval()
	return t.lockedAction("", options.OptsLocking, "stop", func() error {
		return t.lockedStop(options)
	})

}

func (t *Base) lockedStop(options OptsStop) error {
	if err := t.masterStop(options); err != nil {
		return err
	}
	if err := t.slaveStop(options); err != nil {
		return err
	}
	return nil
}

func (t *Base) masterStop(options OptsStop) error {
	return t.action(objectactionprops.Stop, options, func(ctx context.Context, r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("stop resource")
		return resource.Stop(ctx, r)
	})
}

func (t *Base) slaveStop(options OptsStop) error {
	return nil
}
