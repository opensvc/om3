//go:build linux

package netif

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/vishvananda/netlink"
)

func HasCarrier(ifName string) (bool, error) {
	// Labeled interface don't have a separate /sys/class/net/ subdir.
	ifName, _, _ = strings.Cut(ifName, ":")
	p := fmt.Sprintf("/sys/class/net/%s/carrier", ifName)
	b, err := os.ReadFile(p)
	if err != nil {
		return false, err
	}
	return strings.TrimSuffix(string(b), "\n") == "1", nil
}

// IsBridge reports whether the interface is a bridge.
//
// A bridge carries only what its ports carry, so its carrier says whether
// some member interface is up and nothing about the bridge itself. A caller
// deciding whether a carrier reading means anything asks this first.
func IsBridge(ifName string) (bool, error) {
	ifName, _, _ = strings.Cut(ifName, ":")
	p := fmt.Sprintf("/sys/class/net/%s/bridge", ifName)
	fi, err := os.Stat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return fi.IsDir(), nil
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

func InterfaceNameByIP(ref *net.IPNet) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		ips, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, ip := range ips {
			if ref.String() == ip.String() {
				return iface.Name, nil
			}
		}
	}
	return "", nil
}
