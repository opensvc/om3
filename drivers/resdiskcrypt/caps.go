//go:build linux

package resdiskcrypt

import (
	"context"
	"os/exec"

	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	if _, err := exec.LookPath(cryptsetup); err != nil {
		return []string{}, nil
	}
	return []string{drvID.Cap()}, nil
}
