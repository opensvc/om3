//go:build !linux

package resiproute

import (
	"context"

	"github.com/opensvc/om3/core/status"
)

// Start the Resource
func (t *T) Start(_ context.Context) error {
	return nil
}

// Stop the Resource
func (t *T) Stop(_ context.Context) error {
	return nil
}

// Status evaluates and display the Resource status and logs
func (t *T) Status(ctx context.Context) status.T {
	//r.Log.Error("not implemented on %s", runtime.GOOS)
	return status.NotApplicable
}
