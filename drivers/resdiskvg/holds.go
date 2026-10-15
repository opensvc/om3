package resdiskvg

import (
	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the volume group, and the physical volumes it is made of.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	l := make([]resource.Hold, 0)
	if name, ok := src.Get("name"); ok {
		l = append(l, resource.HoldsOf(resource.HoldVG, name)...)
	}
	if pvs, ok := src.Get("pvs"); ok {
		l = append(l, resource.HoldsOf(resource.HoldBlockDevice, resource.DevicePaths(pvs)...)...)
	}
	return l
}
