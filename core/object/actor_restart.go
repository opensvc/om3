package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
)

func (t *actor) stopForRestart(ctx context.Context) error {
	ac := actioncontext.Stop
	ac.Freeze = false
	ctx = actioncontext.WithProps(ctx, ac)
	return t.stopWithContext(ctx)
}

func (t *actor) stopWithContext(ctx context.Context) error {
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

// Restart stops then starts the local instance of the object
func (t *actor) Restart(ctx context.Context) error {
	if err := t.stopForRestart(ctx); err != nil {
		return err
	}
	if err := t.Start(ctx); err != nil {
		return err
	}
	return nil
}
