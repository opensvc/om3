package object

import "opensvc.com/opensvc/core/xconfig"

// ValidateConfig
func (t *Node) ValidateConfig() (xconfig.ValidateAlerts, error) {
	return t.config.Validate()
}
