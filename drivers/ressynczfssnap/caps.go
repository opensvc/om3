package ressynczfs

import (
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/zfs"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	baseCap := drvID.Cap()
	l := make([]string, 0)
	if zfs.IsCapable() {
		l = append(l, baseCap)
	}
	return l, nil
}
