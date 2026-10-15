package rescontainervbox

import (
	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the name of the virtual machine.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	name, _ := src.Get("name")
	return resource.HoldsOf(resource.HoldContainerName, name)
}
