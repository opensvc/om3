package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/statusbus"
)

func (t *actor) Set(ctx context.Context, kops ...keyop.T) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	return t.core.Set(ctx, kops...)
}
