package claimpool

import (
	"context"

	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

// capabilitiesScanner always reports the capability. A claim on a pool is a
// statement about what a namespace may take, which every node can hold and
// read whether or not it can reach the pool itself.
func capabilitiesScanner(ctx context.Context) ([]string, error) {
	return []string{drvID.Cap()}, nil
}
