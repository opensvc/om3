package resdiskloop

import (
	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/loop"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	if !loop.IsCapable() {
		return []string{}, nil
	}
	return []string{drvID.Cap()}, nil
}
