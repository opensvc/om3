//go:build linux

package resdiskdrbd

import (
	"github.com/opensvc/om3/util/drbd"
)

func (t *T) drbd() DRBDDriver {
	d := drbd.New(
		t.Res,
		drbd.WithLogger(t.Log()),
	)
	_ = d.ModProbe()
	return d
}
