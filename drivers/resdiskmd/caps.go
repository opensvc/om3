//go:build linux

package resdiskmd

import (
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/md"
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
