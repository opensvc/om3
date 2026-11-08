//go:build !solaris

package resip

import (
	"net"

	"github.com/j-keck/arping"
)

// gratuitousARP sends a gratuitous arp of the ipv4 address over dev, so the
// neighbors of the link learn the address is on the mac of dev.
func gratuitousARP(ip net.IP, dev string) error {
	return arping.GratuitousArpOverIfaceByName(ip, dev)
}
