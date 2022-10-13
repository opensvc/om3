package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/resource"
)

// Start starts the local instance of the object
func (t *actor) PRStart(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Start)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("start", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedPRStart(ctx)
}

func (t *actor) lockedPRStart(ctx context.Context) error {
	if err := t.masterPRStart(ctx); err != nil {
		return err
	}
	if err := t.slavePRStart(ctx); err != nil {
		return err
	}
	return nil
}

func (t *actor) masterPRStart(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("start resource")
		return resource.PRStart(ctx, r)
	})
}

func (t *actor) slavePRStart(ctx context.Context) error {
	return nil
}
