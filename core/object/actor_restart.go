package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/freeze"
)

// Restart stops then starts the local instance of the object
func (t *actor) Restart(ctx context.Context) error {
	initialFrozenAt := freeze.Frozen(t.path.FrozenFile())

	if err := t.Stop(ctx); err != nil {
		return err
	}
	if err := t.Start(ctx); err != nil {
		return err
	}
	if initialFrozenAt.IsZero() {
		return freeze.Unfreeze(t.path.FrozenFile())
	}
	return nil
}
