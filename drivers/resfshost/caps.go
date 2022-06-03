package resfshost

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/filesystems"
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
