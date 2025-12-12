package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
)

// PRStart starts the scsi reservations of the local instance of the object
func (t *actor) PRStart(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.PRStart)
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
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Attr("rid", r.RID()).Tracef("start resource")
		return resource.PRStart(ctx, r)
	})
}
