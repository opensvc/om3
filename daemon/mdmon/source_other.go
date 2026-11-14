//go:build !linux

package mdmon

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/daemon/evwatch"
	"github.com/opensvc/om3/v3/util/plog"
)

// source is the md event source, of linux only, as the md arrays.
type source struct{}

func newSource(*plog.Logger) *source {
	return &source{}
}

// Name implements evwatch.Source.
func (t *source) Name() string {
	return "md events"
}

// Check implements evwatch.Source: the md arrays are linux's.
func (t *source) Check(context.Context) error {
	return fmt.Errorf("md arrays are linux only: %w", evwatch.ErrUnsupported)
}

// Run implements evwatch.Source.
func (t *source) Run(context.Context, func(), func(string)) error {
	return fmt.Errorf("md arrays are linux only: %w", evwatch.ErrUnsupported)
}
