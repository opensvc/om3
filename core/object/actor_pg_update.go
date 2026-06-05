package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
)

// PGUpdate updates the pg settings. This is usually done on start,
// but users may want to do this precise action after changing a
// pg_* keyword value.
func (t *actor) PGUpdate(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.PGUpdate)
	if err := t.validateAction(); err != nil {
		return err
	}
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedPGUpdate(ctx)
}

func (t *actor) lockedPGUpdate(ctx context.Context) error {
	return t.action(ctx, resource.PGUpdate)
}
