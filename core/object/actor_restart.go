package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
)

func (t *actor) stopForRestart(ctx context.Context) error {
	ac := actioncontext.Stop
	// the stop half of a restart is not a stop the operator wants to stick:
	// the start that follows is the point.
	ac.MarksStopped = false
	ctx = actioncontext.WithProps(ctx, ac)
	return t.stopWithContext(ctx)
}

func (t *actor) stopWithContext(ctx context.Context) error {
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("stop", false)
	if actioncontext.IsInterruptSyncs(ctx) {
		// The syncs running hold the object lock the stop needs: they
		// are ended instead of waited for.
		t.interruptSyncs()
	}
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedStop(ctx)
}

// Restart stops then starts the local instance of the object
//
// The stop and the start report to the collector as one restart.
func (t *actor) Restart(ctx context.Context) (err error) {
	ctx, done := t.beginCollectorAction(ctx, "restart")
	defer func() { done(err) }()
	if err := t.stopForRestart(ctx); err != nil {
		return err
	}
	if err := t.Start(ctx); err != nil {
		return err
	}
	return nil
}
