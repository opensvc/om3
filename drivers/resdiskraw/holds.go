package resdiskraw

import (
	"strings"

	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the source and destination devices of each devs entry naming
// them as paths: an entry naming a volume or a resource is held by it.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	devs, ok := src.Get("devs")
	if !ok {
		return nil
	}
	l := make([]resource.Hold, 0)
	for _, entry := range strings.Fields(devs) {
		l = append(l, resource.HoldsOf(resource.HoldBlockDevice, resource.DevicePaths(strings.ReplaceAll(entry, ":", " "))...)...)
	}
	return l
}
