package object

import "opensvc.com/opensvc/core/xconfig"

// ValidateConfig
func (t *Node) ValidateConfig(options OptsValidateConfig) (xconfig.ValidateAlerts, error) {
	return t.config.Validate()
}
