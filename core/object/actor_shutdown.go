package object

import (
	"context"
	"errors"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
)

// Shutdown shuts down the local instance of the object
func (t *actor) Shutdown(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Shutdown)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("shutdown", false)
	// A shutdown, as the one of a drained node, does not wait for the
	// syncs running, nor leaves them sending while a peer takes over.
	t.interruptSyncs()
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedShutdown(ctx)
}

func (t *actor) lockedShutdown(ctx context.Context) error {
	if err := t.masterShutdown(ctx); err != nil && !errors.Is(err, ErrDisabled) {
		return err
	}
	return nil
}

func (t *actor) masterShutdown(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Attr("rid", r.RID()).Tracef("shutdown resource")
		return resource.Shutdown(ctx, r)
	})
}
