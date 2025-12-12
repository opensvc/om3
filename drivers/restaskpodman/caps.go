package restaskpodman

import (
	"github.com/opensvc/om3/v3/drivers/rescontainerpodman"
	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	drvCap := DrvID.Cap()
	if !rescontainerpodman.IsGenuine() {
		return l, nil
	}
	l = append(l, drvCap)
	return l, nil
}
