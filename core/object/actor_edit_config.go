package object

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/statusbus"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/file"
)

func (t *actor) RecoverAndEditConfig() error {
	ctx := context.Background()
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	return file.Edit(t.ConfigFile(), file.EditModeRecover, func(dst string) error {
		return xconfig.ValidateReferrer(dst, t.config.Referrer)
	})
}

func (t *actor) DiscardAndEditConfig() error {
	ctx := context.Background()
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	return file.Edit(t.ConfigFile(), file.EditModeDiscard, func(dst string) error {
		return xconfig.ValidateReferrer(dst, t.config.Referrer)
	})
}

func (t *actor) EditConfig() error {
	ctx := context.Background()
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	return file.Edit(t.ConfigFile(), file.EditModeNormal, func(dst string) error {
		return xconfig.ValidateReferrer(dst, t.config.Referrer)
	})
}
