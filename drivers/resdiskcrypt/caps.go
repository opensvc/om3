//go:build linux

package resdiskcrypt

import (
	"os/exec"

	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	if _, err := exec.LookPath(cryptsetup); err != nil {
		return []string{}, nil
	}
	return []string{drvID.Cap()}, nil
}
