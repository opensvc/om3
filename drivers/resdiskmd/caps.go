//go:build linux

package resdiskmd

import (
	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/md"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	if !md.IsCapable() {
		return []string{}, nil
	}
	return []string{drvID.Cap()}, nil
}
