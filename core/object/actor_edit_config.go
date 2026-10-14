package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/statusbus"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/file"
)

func (t *actor) RecoverAndEditConfig() error {
	ctx := context.Background()
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	return file.Edit(t.ConfigFile(), file.EditModeRecover, func(dst string) error {
		return t.validateEdit(dst)
	})
}

func (t *actor) DiscardAndEditConfig() error {
	ctx := context.Background()
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	return file.Edit(t.ConfigFile(), file.EditModeDiscard, func(dst string) error {
		return t.validateEdit(dst)
	})
}

func (t *actor) EditConfig() error {
	ctx := context.Background()
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()
	defer t.postActionStatusEval(ctx)
	return file.Edit(t.ConfigFile(), file.EditModeNormal, func(dst string) error {
		return t.validateEdit(dst)
	})
}

// SetEditForce has the next edits of the configuration written even when a
// resource up on this node would leak by them, as root forces it.
func (t *actor) SetEditForce(v bool) {
	t.editForce = v
}

// validateEdit validates the configuration edited at dst, and refuses it
// when a resource up on this node would leak by it, unless forced.
func (t *actor) validateEdit(dst string) error {
	if err := xconfig.ValidateReferrer(dst, t.config.Referrer); err != nil {
		return err
	}
	if t.editForce {
		return nil
	}
	return CheckLocalFileStopToChange(t.path, dst)
}
