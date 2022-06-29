package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/resource"
)

type OptsSetUnprovisioned struct {
	OptsResourceSelector
	OptsLock
	OptDryRun
}

// SetUnprovisioned starts the local instance of the object
func (t *core) SetUnprovisioned(options OptsSetUnprovisioned) error {
	props := actioncontext.SetUnprovisioned
	ctx := context.Background()
	ctx = actioncontext.WithOptions(ctx, options)
	ctx = actioncontext.WithProps(ctx, props)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("set unprovisioned", false)
	unlock, err := t.lockAction(props, options.OptsLock)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedSetUnprovisioned(ctx)
}

func (t *core) lockedSetUnprovisioned(ctx context.Context) error {
	if err := t.masterSetUnprovisioned(ctx); err != nil {
		return err
	}
	if err := t.slaveSetUnprovisioned(ctx); err != nil {
		return err
	}
	return nil
}

func (t *core) masterSetUnprovisioned(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return resource.SetUnprovisioned(ctx, r)
	})
}

func (t *core) slaveSetUnprovisioned(ctx context.Context) error {
	return nil
}
