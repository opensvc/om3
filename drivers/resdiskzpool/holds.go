package resdiskzpool

import (
	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the pool, and the devices of its vdevs.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	l := make([]resource.Hold, 0)
	if name, ok := src.Get("name"); ok {
		l = append(l, resource.HoldsOf(resource.HoldZpool, name)...)
	}
	if vdev, ok := src.Get("vdev"); ok {
		l = append(l, resource.HoldsOf(resource.HoldBlockDevice, resource.DevicePaths(vdev)...)...)
	}
	return l
}
