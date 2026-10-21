package resdiskdisk

import (
	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the disk, once its provision recorded its id.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	id, _ := src.Get("disk_id")
	return resource.HoldsOf(resource.HoldDiskID, id)
}
