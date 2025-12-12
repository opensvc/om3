package object

import (
	"context"
	"errors"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
)

// Run starts the local instance of the object
func (t *actor) Run(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Run)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("run", false)
	if err := t.masterRun(ctx); err != nil {
		return err
	}
	return nil
}

func (t *actor) masterRun(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Attr("rid", r.RID()).Tracef("%s: run resource", r.RID())
		err := resource.Run(ctx, r)
		if errors.Is(err, resource.ErrActionReqNotMet) && actioncontext.IsCron(ctx) {
			return nil
		}
		return err
	})
}
