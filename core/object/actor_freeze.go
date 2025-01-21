package object

import (
	"context"
	"time"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/freeze"
	"github.com/opensvc/om3/core/statusbus"
)

// Frozen returns the unix timestamp of the last freeze.
func (t *actor) Frozen() time.Time {
	return freeze.Frozen(t.path.FrozenFile())
}

// Freeze creates a persistent flag file that prevents orchestration
// of the object instance.
func (t *actor) Freeze(ctx context.Context) error {
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	ctx = actioncontext.WithProps(ctx, actioncontext.Freeze)
	defer t.postActionStatusEval(ctx)
	if err := freeze.Freeze(t.path.FrozenFile()); err != nil {
		return err
	}
	t.log.Infof("now frozen")
	return nil
}

// Unfreeze removes the persistent flag file that prevents orchestration
// of the object instance.
func (t *actor) Unfreeze(ctx context.Context) error {
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	ctx = actioncontext.WithProps(ctx, actioncontext.Unfreeze)
	defer t.postActionStatusEval(ctx)
	if err := freeze.Unfreeze(t.path.FrozenFile()); err != nil {
		return err
	}
	t.log.Infof("now unfrozen")
	return nil
}
