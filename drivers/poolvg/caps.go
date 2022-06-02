package poolvg

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	volDrvID := driver.NewID(driver.GroupVolume, drvID.Name)
	return []string{drvID.Cap(), volDrvID.Cap()}, nil
}
