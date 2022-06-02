package resdiskraw

import (
	"os/exec"

	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/raw"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	if !raw.IsCapable() {
		return []string{}, nil
	}
	if _, err := exec.LookPath("mknod"); err != nil {
		return []string{}, nil
	}
	return []string{drvID.Cap()}, nil
}
