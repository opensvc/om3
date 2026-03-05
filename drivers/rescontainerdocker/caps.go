package rescontainerdocker

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/opensvc/om3/v3/util/capabilities"
)

type (
	dockerInfo struct {
		ClientInfo struct {
			Version string
		}
	}
)

var (
	drvCap           = DrvID.Cap()
	capRegistryCreds = drvCap + ".registry_creds"
	capSignal        = drvCap + ".signal"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func IsGenuine(ctx context.Context) bool {
	b, err := exec.CommandContext(ctx, "docker", "--version").Output()
	if err != nil {
		return false
	} else if bytes.Contains(b, []byte("Docker")) {
		return true
	}
	return false
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	l := make([]string, 0)
	if !IsGenuine(ctx) {
		return l, nil
	}
	l = append(l, drvCap)
	l = append(l, capRegistryCreds)
	l = append(l, capSignal)

	return l, nil
}
