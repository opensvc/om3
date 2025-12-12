package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/xconfig"
)

// ValidateConfig validates the configuration
func (t *core) ValidateConfig(ctx context.Context) (xconfig.Alerts, error) {
	ctx = actioncontext.WithProps(ctx, actioncontext.ValidateConfig)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return xconfig.Alerts{}, err
	}
	defer unlock()
	return t.config.Validate()
}
