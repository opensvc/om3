//go:build linux

package resipnetns

import (
	"net"

	"github.com/j-keck/arping"
)

func (t *T) arpGratuitous(ipaddr net.IP, dev string) error {
	return arping.GratuitousArpOverIfaceByName(ipaddr, dev)
}
