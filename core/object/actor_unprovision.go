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

	t.setenv("unprovision", actioncontext.IsLeader(ctx))
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	if !unprovision {
		// An instance a sysadmin unprovisioned, and declared so, has
		// nothing left to free: the purge of the object goes on to its
		// delete. Judged under the action lock, from the records a
		// provision and a declaration write, not from what a status finds.
		if !t.anyProvisionRecorded() {
			t.log.Infof("unprovision is disabled (unprovision=false), and no resource is recorded provisioned: nothing to do")
			return nil
		}
		return fmt.Errorf("unprovision is disabled (unprovision=false): unprovision the resources by hand, then declare it with 'instance unprovision --state-only'")
	}
	return t.lockedUnprovision(ctx)
}

// anyProvisionRecorded says a resource of the instance is recorded made by
// a provision, or declared made by a sysadmin.
func (t *actor) anyProvisionRecorded() bool {
	for _, r := range t.Resources() {
		if r.IsDisabled() {
			continue
		}
		if resource.ProvisionRecorded(r) {
			return true
		}
	}
	return false
}

// allProvisionRecorded says every resource of the instance is recorded made
// by a provision, or declared made by a sysadmin.
func (t *actor) allProvisionRecorded() bool {
	for _, r := range t.Resources() {
		if r.IsDisabled() {
			continue
		}
		if !resource.ProvisionRecorded(r) {
			return false
		}
	}
	return true
}

func (t *actor) lockedUnprovision(ctx context.Context) error {
	return t.action(ctx, func(ctx context.Context, r resource.Driver) error {
		t.log.Attr("rid", r.RID()).Tracef("unprovision resource")
		leader := actioncontext.IsLeader(ctx)
		return resource.Unprovision(ctx, r, leader)
	})
}
