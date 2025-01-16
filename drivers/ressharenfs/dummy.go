//go:build !linux

package ressharenfs

import (
	"context"
)

func (t *T) stop() error {
	return nil
}

func (t *T) start(_ context.Context) error {
	return nil
}

func (t *T) isUp() (bool, error) {
	return false, nil
}

func (t *T) isPathExported() (bool, error) {
	return false, nil
}
