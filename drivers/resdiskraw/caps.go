package resdiskraw

import (
	"context"
	"os/exec"

	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/raw"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	if !raw.IsCapable() {
		return []string{}, nil
	}
	if _, err := exec.LookPath("mknod"); err != nil {
		return []string{}, nil
	}
	return []string{drvID.Cap()}, nil
}
