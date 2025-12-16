//go:build linux || solaris

package poolzpool

import (
	"context"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/zfs"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	volDrvID := driver.NewID(driver.GroupVolume, drvID.Name)
	if zfs.IsCapable() {
		return []string{drvID.Cap(), volDrvID.Cap()}, nil
	}
	return []string{}, nil
}
