package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/keyop"
)

// Set changes or adds a keyword and its value in the configuration file.
func (t *core) Set(ctx context.Context, kops ...keyop.T) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.config.Set(kops...)
}
