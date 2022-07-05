package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/key"
)

// Unset gets a keyword value
func (t *core) Unset(ctx context.Context, kws ...key.T) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Unset)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return unsetKeys(t.config, kws...)
}

func unsetKeys(cf *xconfig.T, kws ...key.T) error {
	if changes := cf.Unset(kws...); changes > 0 {
		return cf.Commit()
	}
	return nil
}
