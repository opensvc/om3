// +build linux

package netif

import (
	"fmt"
	"net"
	"strings"

	"github.com/vishvananda/netlink"
	"opensvc.com/opensvc/util/file"
)

func HasCarrier(ifName string) (bool, error) {
	p := fmt.Sprintf("/sys/class/net/%s/carrier", ifName)
	b, err := file.ReadAll(p)
	if err != nil {
		return false, err
	}
	return strings.TrimSuffix(string(b), "\n") == "1", nil
}

func AddAddr(ifName string, ipnet *net.IPNet) error {
	addr := &netlink.Addr{IPNet: ipnet}
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return err
	}
	err = netlink.AddrAdd(link, addr)
	if err != nil {
		return err
	}
	return nil
}

func DelAddr(ifName string, ipnet *net.IPNet) error {
	addr := &netlink.Addr{IPNet: ipnet}
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return err
	}
	err = netlink.AddrDel(link, addr)
	if err != nil {
		return err
	}
	return nil
}
