package restaskdocker

import (
	"os/exec"

	"github.com/opensvc/om3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	drvCap := drvID.Cap()
	if _, err := exec.LookPath("docker"); err != nil {
		return l, nil
	}
	l = append(l, drvCap)
	l = append(l, altDrvID.Cap())
	return l, nil
}
