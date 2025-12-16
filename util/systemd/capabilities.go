package systemd

import (
	"context"

	"github.com/opensvc/om3/v3/util/capabilities"
)

const (
	NodeCapability = "node.x.systemd"
)

// capabilitiesScanner is the capabilities scanner for systemd
func capabilitiesScanner(ctx context.Context) ([]string, error) {
	if HasSystemd() {
		return []string{NodeCapability}, nil
	}
	return nil, nil
}

// register node scanners
func init() {
	capabilities.Register(capabilitiesScanner)
}
