package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/resource"
)

type OptsSetUnprovisioned struct {
	OptsGlobal
	OptsResourceSelector
	OptsLocking
}

// SetUnprovisioned starts the local instance of the object
func (t *Base) SetUnprovisioned(options OptsSetUnprovisioned) error {
	ctx := actioncontext.New(options, objectactionprops.SetUnprovisioned)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("set unprovisioned", false)
	err := t.lockedAction("", options.OptsLocking, "set unprovisioned", func() error {
		return t.lockedSetUnprovisioned(ctx)
	})
	return err
}

func (t *Base) lockedSetUnprovisioned(ctx context.Context) error {
	if err := t.masterSetUnprovisioned(ctx); err != nil {
		return err
	}
	if err := t.slaveSetUnprovisioned(ctx); err != nil {
		return err
	}
	return nil
}

func (t *Base) masterSetUnprovisioned(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return resource.SetUnprovisioned(ctx, r)
	})
}

func (t *Base) slaveSetUnprovisioned(ctx context.Context) error {
	return nil
}
