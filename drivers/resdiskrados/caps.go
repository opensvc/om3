package resdiskrados

import (
	"context"
	"os/exec"

	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	l := make([]string, 0)
	if _, err := exec.LookPath("rbd"); err == nil {
		l = append(l, drvID.Cap())
	}
	return l, nil
}
