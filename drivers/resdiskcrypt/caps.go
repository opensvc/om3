package resdiskcrypt

import (
	"os/exec"

	"opensvc.com/opensvc/util/capabilities"
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
