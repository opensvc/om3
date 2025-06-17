package object

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/resource"
)

// SetUnprovisioned starts the local instance of the object
func (t *actor) SetUnprovisioned(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.SetUnprovisioned)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("set unprovisioned", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedSetUnprovisioned(ctx)
}

func (t *actor) lockedSetUnprovisioned(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return resource.SetUnprovisioned(ctx, r)
	})
}
