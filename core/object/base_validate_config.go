package object

import (
	"opensvc.com/opensvc/core/objectactionprops"
	"opensvc.com/opensvc/core/xconfig"
)

// OptsValidateConfig is the options of the ValidateConfig object method.
type OptsValidateConfig struct {
	OptsLock
}

// ValidateConfig
func (t *Base) ValidateConfig(options OptsValidateConfig) (xconfig.ValidateAlerts, error) {
	props := objectactionprops.ValidateConfig
	unlock, err := t.lockAction(props, options.OptsLock)
	if err != nil {
		return xconfig.ValidateAlerts{}, err
	}
	defer unlock()
	return t.config.Validate()
}
