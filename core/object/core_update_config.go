package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/util/key"
)

// Update updates applies configurations changes in the configuration file.
func (t *core) Update(ctx context.Context, deleteSections []string, unsetKeys []key.T, keyOps []keyop.T) (err error) {
	ctx = actioncontext.WithProps(ctx, actioncontext.Set)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.config.Update(deleteSections, unsetKeys, keyOps)
}
