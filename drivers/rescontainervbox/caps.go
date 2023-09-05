package rescontainervbox

import (
	"os/exec"

	"github.com/opensvc/om3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	if _, err := exec.LookPath("VBoxManage"); err == nil {
		l = append(l, drvID.Cap())
	}
	return l, nil
}
