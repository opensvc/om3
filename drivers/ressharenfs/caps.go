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
	exportfsPath, err := exec.LookPath("exportfs")
	if err != nil {
		return []string{}, nil
	}
	showmountPath, err := exec.LookPath("showmount")
	if err != nil {
		return []string{}, nil
	}
	caps := []string{
		capabilities.MakePath("exportfs", exportfsPath),
		capabilities.MakePath("showmount", showmountPath),
		drvID.Cap(),
	}
	return caps, nil
}
