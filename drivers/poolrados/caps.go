package poolrados

import (
	"context"
	"os/exec"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	volDrvID := driver.NewID(driver.GroupVolume, drvID.Name)
	if _, err := exec.LookPath("ceph"); err == nil {
		return []string{drvID.Cap(), volDrvID.Cap()}, nil
	}
	return []string{}, nil
}
