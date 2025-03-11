package rescontaineroci

import (
	"github.com/opensvc/om3/drivers/rescontainerdocker"
	"github.com/opensvc/om3/drivers/rescontainerpodman"
	"github.com/opensvc/om3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	if rescontainerdocker.IsGenuine() {
		l = append(l, drvID.Cap())
	} else if rescontainerpodman.IsGenuine() {
		l = append(l, drvID.Cap())
	}
	return l, nil
}
