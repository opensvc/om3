package object

import (
	"context"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/xconfig"
)

// ValidateConfig
func (t *core) ValidateConfig(ctx context.Context) (xconfig.ValidateAlerts, error) {
	ctx = actioncontext.WithProps(ctx, actioncontext.ValidateConfig)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return xconfig.ValidateAlerts{}, err
	}
	defer unlock()
	return t.config.Validate()
}
