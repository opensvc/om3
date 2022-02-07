package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/resource"
)

type OptsSetProvisioned struct {
	OptsGlobal
	OptsResourceSelector
	OptsLocking
}

// SetProvisioned starts the local instance of the object
func (t *Base) SetProvisioned(options OptsSetProvisioned) error {
	ctx := actioncontext.New(options, objectactionprops.SetProvisioned)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("set provisioned", false)
	err := t.lockedAction("", options.OptsLocking, "set provisioned", func() error {
		return t.lockedSetProvisioned(ctx)
	})
	return err
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
