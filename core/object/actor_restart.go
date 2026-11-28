package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
)

// stopForRestartProps are the properties of the stop half of a restart.
func stopForRestartProps() actioncontext.Properties {
	ac := actioncontext.Stop
	// the stop half of a restart is not a stop the operator wants to stick:
	// the start that follows is the point.
	ac.MarksStopped = false
	// The process groups are kept, and the systemd properties of their
	// slices with them: the start makes them again a moment later, and
	// sets every cap of the configuration. Removing them is wasted work,
	// and on the cgroup v1 hierarchy it stops their slices, which a start
	// coming before systemd has run the stop loses file-only caps to.
	// A cap whose keyword was removed stays until a stop, as it stays
	// under a running instance: the default value lifts it.
	ac.RemovesPG = false
	return ac
}

func (t *actor) stopForRestart(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, stopForRestartProps())
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
	defer func() { done(err, recover()) }()
	if err := t.stopForRestart(ctx); err != nil {
		return err
	}
	if err := t.Start(ctx); err != nil {
		return err
	}
	return nil
}
