package rescontainerocibase

import (
	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the name of the container, the one it is given or the one made
// of the object path and the rid: two objects naming the same, or a clone of
// one naming its container, would run one container.
func (t *BT) Holds(src resource.HoldSource) []resource.Hold {
	name, ok := src.Get("name")
	if !ok {
		return nil
	}
	return resource.HoldsOf(resource.HoldContainerName, containerName(name, src.Path(), src.RID()))
}
