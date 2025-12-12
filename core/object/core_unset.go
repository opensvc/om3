package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/util/key"
)

// Unset unsets keywords
func (t *core) Unset(ctx context.Context, kws ...key.T) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Unset)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.config.Unset(kws...)
}
