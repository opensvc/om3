package object

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/provisioned"
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
		// An instance a sysadmin unprovisioned, and declared so, has
		// nothing left to free: the purge of the object goes on to its
		// delete.
		if st, err := t.FreshStatus(ctx); err == nil && st.Provisioned == provisioned.False {
			t.log.Infof("unprovision is disabled (unprovision=false), and the instance is not provisioned: nothing to do")
			return nil
		}
		return fmt.Errorf("unprovision is disabled (unprovision=false): unprovision the resources by hand, then declare it with 'instance unprovision --state-only'")
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
