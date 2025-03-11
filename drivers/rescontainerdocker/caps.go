package rescontainerdocker

import (
	"bytes"
	"os/exec"

	"github.com/opensvc/om3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func IsGenuine() bool {
	b, err := exec.Command("docker", "--version").Output()
	if err != nil {
		return false
	} else if bytes.Contains(b, []byte("Docker")) {
		return true
	}
	return false
}

func capabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	drvCap := DrvID.Cap()
	if !IsGenuine() {
		return l, nil
	}
	l = append(l, drvCap)
	l = append(l, drvCap+".registry_creds")
	l = append(l, drvCap+".signal")
	return l, nil
}
