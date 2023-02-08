package object

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/statusbus"
	"github.com/opensvc/om3/core/xconfig"
)

func (t *actor) RecoverAndEditConfig() error {
	ctx := context.Background()
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	return xconfig.Edit(t.ConfigFile(), xconfig.EditModeRecover, t.config.Referrer)
}

func (t *actor) DiscardAndEditConfig() error {
	ctx := context.Background()
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	return xconfig.Edit(t.ConfigFile(), xconfig.EditModeDiscard, t.config.Referrer)
}

func (t *actor) EditConfig() error {
	ctx := context.Background()
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	return xconfig.Edit(t.ConfigFile(), xconfig.EditModeNormal, t.config.Referrer)
}
