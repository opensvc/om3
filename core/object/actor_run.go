package object

import (
	"context"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/resource"
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
	if err := t.slaveRun(ctx); err != nil {
		return err
	}
	return nil
}

func (t *actor) masterRun(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("run resource")
		err := resource.Run(ctx, r)
		if err == nil {
			return nil
		}
		if errors.Is(err, resource.ErrReqNotMet) && actioncontext.IsCron(ctx) {
			return nil
		}
		return err
	})
}

func (t *actor) slaveRun(ctx context.Context) error {
	return nil
}
