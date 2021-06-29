package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resourceselector"
)

// OptsUnprovision is the options of the Unprovision object method.
type OptsUnprovision struct {
	OptsGlobal
	OptsAsync
	OptsLocking
	resourceselector.Options
	OptTo
	OptForce
	OptLeader
}

// Unprovision stops and frees the local instance of the object
func (t *Base) Unprovision(options OptsUnprovision) error {
	ctx := actioncontext.New(options, objectactionprops.Unprovision)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("unprovision", false)
	defer t.postActionStatusEval()
	return t.lockedAction("", options.OptsLocking, "unprovision", func() error {
		return t.lockedUnprovision(ctx)
	})
}

func (t *Base) lockedUnprovision(ctx context.Context) error {
	if err := t.slaveUnprovision(ctx); err != nil {
		return err
	}
	if err := t.masterUnprovision(ctx); err != nil {
		return err
	}
	return nil
}

func (t *Base) masterUnprovision(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("unprovision resource")
		leader := actioncontext.IsLeader(ctx)
		return resource.Unprovision(ctx, r, leader)
	})
}

func (t *Base) slaveUnprovision(ctx context.Context) error {
	return nil
}
