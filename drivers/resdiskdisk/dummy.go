//go:build !linux

package resdiskdisk

import (
	"context"

	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/device"
)

func (t *T) Status(ctx context.Context) status.T {
	return status.NotApplicable
}

func (t T) ExposedDevices() device.L {
	l := make(device.L, 0)
	return l
}

func (t T) unconfigure() error {
	return nil
}
func (t T) configure(force forceMode) error {
	return nil
}
