package object

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/resource"
)

// Provision allocates and starts the local instance of the object
func (t *actor) Provision(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Provision)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("provision", actioncontext.IsLeader(ctx))
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	if err := t.lockedProvision(ctx); err != nil {
		return err
	}
	if actioncontext.IsRollbackDisabled(ctx) {
		// --disable-rollback handling
		return nil
	}
	return t.lockedStop(ctx)
}

func (t *actor) lockedProvision(ctx context.Context) error {
	if err := t.masterProvision(ctx); err != nil {
		return err
	}
	if err := t.slaveProvision(ctx); err != nil {
		return err
	}
	return nil
}

func (t *actor) masterProvision(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		rid := r.RID()
		t.log.Attr("rid", rid).Debugf("%s: provision resource", rid)
		leader := actioncontext.IsLeader(ctx)
		return resource.Provision(ctx, r, leader)
	})
}

func (t *actor) slaveProvision(ctx context.Context) error {
	return nil
}
