package object

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/key"
)

// Unprovision stops and frees the local instance of the object
func (t *actor) Unprovision(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Unprovision)
	if err := t.validateAction(); err != nil {
		return err
	}
	unprovision := t.config.GetBool(key.New("", "unprovision"))
	if !unprovision {
		return fmt.Errorf("unprovision is disabled: make sure all resources have been unprovisioned by a sysadmin and execute 'instance unprovision --state-only")
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
		t.log.Attr("rid", r.RID()).Tracef("unprovision resource")
		leader := actioncontext.IsLeader(ctx)
		return resource.Unprovision(ctx, r, leader)
	})
}
