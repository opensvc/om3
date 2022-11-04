package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/statusbus"
	"opensvc.com/opensvc/core/xconfig"
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
