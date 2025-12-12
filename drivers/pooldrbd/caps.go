//go:build linux

package pooldrbd

import (
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	volDrvID := driver.NewID(driver.GroupVolume, drvID.Name)
	return []string{drvID.Cap(), volDrvID.Cap()}, nil
}
