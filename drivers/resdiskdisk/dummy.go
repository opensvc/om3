//go:build !linux
// +build !linux

package resdiskdisk

import (
	"context"

	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/device"
)

func (t *T) Status(ctx context.Context) status.T {
	return status.NotApplicable
}

func (t T) ExposedDevices() []*device.T {
	// TODO implement for non Linux
	l := make([]*device.T, 0)
	return l
}
