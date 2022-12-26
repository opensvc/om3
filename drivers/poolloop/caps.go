//go:build linux

package poolloop

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/loop"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	volDrvID := driver.NewID(driver.GroupVolume, drvID.Name)
	if loop.IsCapable() {
		return []string{drvID.Cap(), volDrvID.Cap()}, nil
	}
	return []string{}, nil
}
