package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/pg"
)

// PGReset lifts the capping of the process groups of the instance.
//
// Where PGUpdate writes what the pg_* keywords say, this writes what a node
// that never capped anything holds, whatever the keywords say and whoever
// wrote the capping being lifted. It is the way out of a capping the
// configuration does not know about: one left by an older agent, by systemd,
// or by hand.
//
// A capping the configuration does name comes back at the next update, and at
// the next start. Lifting one for good is a pg_* keyword set to "default".
func (t *actor) PGReset(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.PGReset)
	if err := t.validateAction(); err != nil {
		return err
	}
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedPGReset(ctx)
}

func (t *actor) lockedPGReset(ctx context.Context) error {
	// The slice of this object bounds what the walk may lift. The slices
	// above it, the namespace's and the one holding every object of the
	// node, are registered by the action too, and are not this object's to
	// lift.
	if mgr := pg.FromContext(ctx); mgr != nil {
		mgr.SetResetRoot(t.pgConfig("").ID)
	}
	return t.action(ctx, resource.PGReset)
}
