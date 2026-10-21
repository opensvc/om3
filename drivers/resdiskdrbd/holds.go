package resdiskdrbd

import (
	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the drbd resource.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	res, _ := src.Get("res")
	return resource.HoldsOf(resource.HoldDRBD, res)
}
