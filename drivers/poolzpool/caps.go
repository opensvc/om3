package poolzpool

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/zfs"
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
