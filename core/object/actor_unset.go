package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/statusbus"
	"github.com/opensvc/om3/v3/util/key"
)

// Unset object keywords
func (t *actor) Unset(ctx context.Context, kws ...key.T) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Unset)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.config.Unset(kws...)
}
