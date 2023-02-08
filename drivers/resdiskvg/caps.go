//go:build linux

package resdiskvg

import (
	"github.com/opensvc/om3/util/capabilities"
	"github.com/opensvc/om3/util/lvm2"
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
