//go:build linux || solaris

package poolzpool

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/util/capabilities"
	"github.com/opensvc/om3/util/zfs"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	volDrvID := driver.NewID(driver.GroupVolume, drvID.Name)
	if zfs.IsCapable() {
		return []string{drvID.Cap(), volDrvID.Cap()}, nil
	}
	return []string{}, nil
}
