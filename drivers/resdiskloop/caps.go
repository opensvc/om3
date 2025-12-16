package resdiskloop

import (
	"context"

	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/loop"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	if !loop.IsCapable() {
		return []string{}, nil
	}
	return []string{drvID.Cap()}, nil
}
