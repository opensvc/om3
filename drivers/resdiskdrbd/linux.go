//go:build linux

package resdiskdrbd

import (
	"context"

	"github.com/opensvc/om3/util/drbd"
)

func (t *T) drbd(ctx context.Context) DRBDDriver {
	d := drbd.New(
		t.Res,
		drbd.WithLogger(t.Log()),
	)
	_ = d.ModProbe(ctx)
	return d
}
