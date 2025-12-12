//go:build linux

package resdisklv

import (
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/lvm2"
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
	return l, nil
}
