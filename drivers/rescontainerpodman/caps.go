package rescontainerpodman

import (
	"context"
	"os/exec"

	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func IsGenuine() bool {
	if _, err := exec.LookPath("podman"); err != nil {
		return false
	}
	return true
}

// capabilitiesScanner scans and returns a list of available driver capabilities
// as strings, depending on system tools.
// It conditionally adds capabilities based on the availability of Podman,
// Docker, or Docker-native systems.
func capabilitiesScanner(ctx context.Context) ([]string, error) {
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
