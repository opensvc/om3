package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/statusbus"
)

func (t *actor) Set(ctx context.Context, kops ...keyop.T) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	return t.core.Set(ctx, kops...)
}
