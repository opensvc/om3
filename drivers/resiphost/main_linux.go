package resiphost

import (
	"context"
	"fmt"

	"github.com/vishvananda/netlink"

	"github.com/opensvc/om3/v3/drivers/resip"
)

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	dev, idx := resip.SplitDevLabel(t.Dev)
	s := fmt.Sprintf("%s %s", t.ipnet(), dev)
	if t.Alias && idx == "" {
		// no label to search
		return s
	}
	if idx != "" {
		// forced label
		return fmt.Sprintf("%s label %s", s, t.Dev)
	}
	// lookup the allocated label
	link, err := netlink.LinkByName(dev)
	if err != nil {
		return s
	}

	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return s
	}

	ip := t.ipaddr()
	for _, addr := range addrs {
		if addr.IP.Equal(ip) {
			return fmt.Sprintf("%s label %s", s, addr.Label)
		}
	}
	return s
}
