//go:build linux

package resdiskdrbd

import (
	"context"
	"strings"

	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/drbd"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	caps := make([]string, 0)
	if !drbd.IsCapable(ctx) {
		return caps, nil
	}
	caps = append(caps, drvID.Cap())
	if v, err := drbd.Version(ctx); err != nil {
		return caps, err
	} else if strings.HasPrefix(v, "9.") {
		caps = append(caps, drvID.Cap()+".mesh")
	}
	return caps, nil
}
