package object

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/resource"
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
	if err := t.masterStop(ctx); err != nil {
		return err
	}
	if err := t.slaveStop(ctx); err != nil {
		return err
	}
	return nil
}

func (t *actor) masterStop(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Attr("rid", r.RID()).Debugf("stop resource")
		return resource.Stop(ctx, r)
	})
}

func (t *actor) slaveStop(ctx context.Context) error {
	return nil
}
