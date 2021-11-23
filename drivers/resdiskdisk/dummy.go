// +build !linux

package resdiskraw

import (
	"context"

	"opensvc.com/opensvc/core/status"
)

func (t *T) Status(ctx context.Context) status.T {
	return status.NotApplicable
}
