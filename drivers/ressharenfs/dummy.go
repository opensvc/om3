//go:build !linux
// +build !linux

package ressharenfs

import (
	"opensvc.com/opensvc/util/capabilities"
)

func capabilitiesScanner() ([]string, error) {
	return []string{}, nil
}

func (t T) stop() error {
	return nil
}

func (t T) start() error {
	return nil
}

func (t T) isUp() (bool, error) {
	return false, nil
}
