//go:build linux

package resipcni

import "github.com/opensvc/om3/v3/util/capabilities"

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner() ([]string, error) {
	return []string{drvID.Cap()}, nil
}
