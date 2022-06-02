package ressharenfs

import (
	"os/exec"

	"opensvc.com/opensvc/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	_, err := exec.LookPath("exportfs")
	if err != nil {
		return []string{}, nil
	}
	return []string{drvID.Cap()}, nil
}
