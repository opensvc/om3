package resdiskcrypt

import (
	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the device mapper name, the device it exposes, and the device
// it encrypts.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	l := make([]resource.Hold, 0)
	if name, ok := src.Get("name"); ok && name != "" {
		l = append(l, resource.HoldsOf(resource.HoldDMName, name)...)
		l = append(l, resource.HoldsOf(resource.HoldBlockDevice, "/dev/mapper/"+name)...)
	}
	if dev, ok := src.Get("dev"); ok {
		l = append(l, resource.HoldsOf(resource.HoldBlockDevice, resource.DevicePaths(dev)...)...)
	}
	return l
}
