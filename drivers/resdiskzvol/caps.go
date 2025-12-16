package resdiskzvol

import (
	"context"

	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/zfs"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	l := make([]string, 0)
	if !zfs.IsCapable() {
		return l, nil
	}
	l = append(l, drvID.Cap())
	return l, nil
}
