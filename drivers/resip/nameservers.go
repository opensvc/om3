package resip

import (
	"github.com/opensvc/om3/v3/core/network"
)

// NetworkNameservers returns the nameservers the network named says are
// reachable from it, none when it names none, or is not an om network.
func NetworkNameservers(name string) []string {
	if name == "" {
		return nil
	}
	nw, _, err := network.Lookup(name)
	if err != nil || nw == nil {
		return nil
	}
	if i, ok := nw.(network.Nameserverer); ok {
		return i.Nameservers()
	}
	return nil
}
