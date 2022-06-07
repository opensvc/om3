package resdiskzpool

import (
	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/zfs"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	if !zfs.IsCapable() {
		return l, nil
	}
	l = append(l, drvID.Cap())
	return l, nil
}
