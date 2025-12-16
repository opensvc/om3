package ressharenfs

import (
	"context"
	"os/exec"

	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	_, err := exec.LookPath("exportfs")
	if err != nil {
		return []string{}, nil
	}
	return []string{drvID.Cap()}, nil
}
