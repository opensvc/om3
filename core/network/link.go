package network

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

// DelLink deletes the link named name, a network teardown made, if it exists.
func DelLink(nw Networker, name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return nil
		}
		return fmt.Errorf("link %s: %w", name, err)
	}
	nw.Log().Infof("delete the link %s", name)
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("delete the link %s: %w", name, err)
	}
	return nil
}
