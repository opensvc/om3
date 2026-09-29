package resiprule

import (
	"context"
	"os/exec"
	"runtime"

	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	if runtime.GOOS != "linux" {
		return nil, nil
	}
	for _, name := range []string{"ip", "nsenter"} {
		if _, err := exec.LookPath(name); err != nil {
			return nil, nil
		}
	}
	return []string{drvID.Cap()}, nil
}
