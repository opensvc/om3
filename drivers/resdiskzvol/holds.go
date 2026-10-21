package resdiskzvol

import (
	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the dataset of the volume.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	name, _ := src.Get("name")
	return resource.HoldsOf(resource.HoldZfsDataset, name)
}
