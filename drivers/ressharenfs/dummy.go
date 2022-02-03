//go:build !linux
// +build !linux

package ressharenfs

import (
	"context"
)

func capabilitiesScanner() ([]string, error) {
	return []string{}, nil
}

func (t T) stop() error {
	return nil
}

func (t T) start(_ context.Context) error {
	return nil
}

func (t T) isUp() (bool, error) {
	return false, nil
}

func (t *T) isPathExported() (bool, error) {
	return false, nil
}
