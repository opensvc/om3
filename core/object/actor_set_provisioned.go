package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/resource"
)

type OptsSetProvisioned struct {
	OptsResourceSelector
	OptsLock
	OptDryRun
}

// SetProvisioned starts the local instance of the object
func (t *Base) SetProvisioned(options OptsSetProvisioned) error {
	props := objectactionprops.SetProvisioned
	ctx := context.Background()
	ctx = actioncontext.WithOptions(ctx, options)
	ctx = actioncontext.WithProps(ctx, props)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("set provisioned", false)
	unlock, err := t.lockAction(props, options.OptsLock)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedSetProvisioned(ctx)
}

func (t *Base) lockedSetProvisioned(ctx context.Context) error {
	if err := t.masterSetProvisioned(ctx); err != nil {
		return err
	}
	if err := t.slaveSetProvisioned(ctx); err != nil {
		return err
	}
	return nil
}

func (t *Base) masterSetProvisioned(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return resource.SetProvisioned(ctx, r)
	})
}

func (t *Base) slaveSetProvisioned(ctx context.Context) error {
	return nil
}
