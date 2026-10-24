package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
)

// Unstop lowers the flags a stop raised, as a start does, without starting
// anything: the flag of the instance, which keeps the daemon from starting it
// on its own, and the ones of its resources, which keep it from restarting
// them. With a resource selector, only the flags of the resources selected.
//
// An instance stopped on purpose becomes a candidate for the daemon again,
// as a node of a failover object to take it over the day the one running it
// fails, without running the object now.
func (t *actor) Unstop(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Unstop)
	if err := t.validateAction(); err != nil {
		return err
	}
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.action(ctx, resource.Unstop)
}
