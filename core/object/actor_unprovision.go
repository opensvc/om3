package object

import (
	"context"

	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/resource"
)

// OptsUnprovision is the options of the Unprovision object method.
type OptsUnprovision struct {
	OptsGlobal
	OptsAsync
	OptsLocking
	OptsResourceSelector
	OptForce
	OptLeader
}

// Unprovision stops and frees the local instance of the object
func (t *Base) Unprovision(options OptsUnprovision) error {
	defer t.setActionOptions(options)()
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("unprovision", false)
	defer t.postActionStatusEval()
	return t.lockedAction("", options.OptsLocking, "unprovision", func() error {
		return t.lockedUnprovision(options)
	})
}

func (t *Base) lockedUnprovision(options OptsUnprovision) error {
	if err := t.slaveUnprovision(options); err != nil {
		return err
	}
	if err := t.masterUnprovision(options); err != nil {
		return err
	}
	return nil
}

func (t *Base) masterUnprovision(options OptsUnprovision) error {
	return t.action(objectactionprops.Unprovision, options, func(ctx context.Context, r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("unprovision resource")
		return resource.Unprovision(ctx, r, options.Leader)
	})
}

func (t *Base) slaveUnprovision(options OptsUnprovision) error {
	return nil
}
