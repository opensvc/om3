package resdiskzpool

import (
	"os/exec"

	"opensvc.com/opensvc/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	if _, err := exec.LookPath("zpool"); err != nil {
		return l, nil
	}
	l = append(l, drvID.Cap())
	return l, nil
}
