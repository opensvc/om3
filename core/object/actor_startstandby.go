package object

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/resource"
)

// StartStandby starts the local instance of the object
func (t *actor) StartStandby(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.StartStandby)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("start", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedStartStandby(ctx)
}

func (t *actor) lockedStartStandby(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		return resource.StartStandby(ctx, r)
	})
}
