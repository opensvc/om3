package object

import "github.com/opensvc/om3/core/xconfig"

// ValidateConfig
func (t *Node) ValidateConfig() (xconfig.ValidateAlerts, error) {
	return t.config.Validate()
}
