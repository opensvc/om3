package object

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/resource"
)

// Unprovision stops and frees the local instance of the object
func (t *actor) Unprovision(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Unprovision)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("unprovision", actioncontext.IsLeader(ctx))
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedUnprovision(ctx)
}

func (t *actor) lockedUnprovision(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Attr("rid", r.RID()).Debugf("unprovision resource")
		leader := actioncontext.IsLeader(ctx)
		return resource.Unprovision(ctx, r, leader)
	})
}
