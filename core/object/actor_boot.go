package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
)

// Boot deactivates local instance of the object when the node is rebooted
func (t *actor) Boot(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Boot)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("boot", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedBoot(ctx)
}

func (t *actor) lockedBoot(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Attr("rid", r.RID()).Tracef("boot resource")
		return resource.Boot(ctx, r)
	})
}
