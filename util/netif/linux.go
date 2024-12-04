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
	p := fmt.Sprintf("/sys/class/net/%s/carrier", ifName)
	b, err := os.ReadFile(p)
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

func InterfaceNameByNet(ref *net.IPNet) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			ip := net.ParseIP(strings.Split(addr.String(), "/")[0])
			if ref.Contains(ip) {
				return iface.Name, nil
			}
		}
	}
	return "", nil
}
