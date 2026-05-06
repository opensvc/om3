package resdiskhp3par

import (
	"context"
	"os/exec"

	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	// Check if 3PAR CLI tools are available
	// Try to find the cli command
	if _, err := exec.LookPath("cli"); err == nil {
		return []string{drvID.Cap()}, nil
	}
	// Try ssh as alternative
	if _, err := exec.LookPath("ssh"); err == nil {
		return []string{drvID.Cap()}, nil
	}
	return nil, nil
}
