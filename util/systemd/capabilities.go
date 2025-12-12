package systemd

import "github.com/opensvc/om3/v3/util/capabilities"

const (
	NodeCapability = "node.x.systemd"
)

// CapabilitiesScanner is the capabilities scanner for systemd
func CapabilitiesScanner() ([]string, error) {
	if HasSystemd() {
		return []string{NodeCapability}, nil
	}
	return nil, nil
}

// register node scanners
func init() {
	capabilities.Register(CapabilitiesScanner)
}
