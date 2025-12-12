//go:build linux

package poolloop

import (
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/loop"
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
