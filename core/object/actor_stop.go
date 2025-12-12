package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
)

// Stop stops the local instance of the object
func (t *actor) Stop(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Stop)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("stop", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedStop(ctx)
}

func (t *actor) lockedStop(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return resource.Stop(ctx, r)
	})
}
