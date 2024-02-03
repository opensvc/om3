package object

import "github.com/opensvc/om3/core/xconfig"

// ValidateConfig node configuration
func (t *Node) ValidateConfig() (xconfig.Alerts, error) {
	return t.config.Validate()
}
