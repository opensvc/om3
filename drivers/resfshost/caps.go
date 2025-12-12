package resfshost

import (
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/filesystems"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	l := []string{}
	for _, t := range filesystems.Types() {
		if !filesystems.IsCapable(t) {
			continue
		}
		drvID := driver.NewID(driver.GroupFS, t)
		l = append(l, drvID.Cap())
	}
	return l, nil
}
