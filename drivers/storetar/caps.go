package storetar

import (
	"context"

	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

// capabilitiesScanner says the driver is usable: it needs nothing but the
// standard library.
func capabilitiesScanner(ctx context.Context) ([]string, error) {
	return []string{drvID.Cap()}, nil
}
