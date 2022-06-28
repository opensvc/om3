package object

import "opensvc.com/opensvc/core/xconfig"

// OptsValidateConfig is the options of the ValidateConfig object method.
type OptsValidateConfig struct {
	OptsGlobal
	OptsLocking
}

// ValidateConfig
func (t *Base) ValidateConfig(options OptsValidateConfig) (xconfig.ValidateAlerts, error) {
	return t.config.Validate()
}
