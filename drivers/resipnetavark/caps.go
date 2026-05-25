//go:build linux

package resipnetavark

import (
	"context"

	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/file"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	// Check if netavark binary exists
	bin := ""
	candidates := []string{
		"/usr/lib/podman/netavark",
		"/usr/libexec/podman/netavark",
		"/usr/local/bin/netavark",
		"/usr/bin/netavark",
	}
	for _, s := range candidates {
		if file.Exists(s) {
			bin = s
			break
		}
	}
	if bin == "" {
		return []string{}, nil
	}
	return []string{drvID.Cap()}, nil
}
