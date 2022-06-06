package resdiskvg

import (
	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/lvm2"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	if !lvm2.IsCapable() {
		return l, nil
	}
	l = append(l, drvID.Cap())
	l = append(l, altDrvID.Cap())
	return l, nil
}
