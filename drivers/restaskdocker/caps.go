package restaskdocker

import (
	"github.com/opensvc/om3/v3/drivers/rescontainerdocker"
	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	drvCap := DrvID.Cap()
	if !rescontainerdocker.IsGenuine() {
		return l, nil
	}
	l = append(l, drvCap)
	return l, nil
}
