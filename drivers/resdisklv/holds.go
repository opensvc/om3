package resdisklv

import (
	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the logical volume, and the device it exposes, which a
// filesystem of another object naming it as a path holds too.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	vg, ok1 := src.Get("vg")
	name, ok2 := src.Get("name")
	if !ok1 || !ok2 || vg == "" || name == "" {
		return nil
	}
	l := resource.HoldsOf(resource.HoldLV, vg+"/"+name)
	return append(l, resource.HoldsOf(resource.HoldBlockDevice, "/dev/"+vg+"/"+name)...)
}
