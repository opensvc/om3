package resipnetns

import (
	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the address, as named or drawn from a network.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	name, _ := src.Get("name")
	addr, _ := src.Get("addr")
	return resource.HoldsOf(resource.HoldIPAddress, name, addr)
}
