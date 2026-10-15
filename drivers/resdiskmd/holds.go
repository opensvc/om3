package resdiskmd

import (
	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the array, by its uuid, the device it exposes by its name, and
// its member devices.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	l := make([]resource.Hold, 0)
	if uuid, ok := src.Get("uuid"); ok {
		l = append(l, resource.HoldsOf(resource.HoldMD, uuid)...)
	}
	if name, ok := src.Get("name"); ok && name != "" {
		l = append(l, resource.HoldsOf(resource.HoldBlockDevice, "/dev/md/"+name)...)
	}
	if devs, ok := src.Get("devs"); ok {
		l = append(l, resource.HoldsOf(resource.HoldBlockDevice, resource.DevicePaths(devs)...)...)
	}
	return l
}
