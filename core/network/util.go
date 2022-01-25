package network

import (
	"net"

	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/util/key"
)

func sectionName(networkName string) string {
	return "network#" + networkName
}

func cKey(networkName string, option string) key.T {
	section := sectionName(networkName)
	return key.New(section, option)
}

func cString(config *xconfig.T, networkName string, option string) string {
	network := cKey(networkName, option)
	return config.GetString(network)
}

func pKey(p Networker, s string) key.T {
	return key.New("network#"+p.Name(), s)
}

func IncIPN(ip net.IP, n int) {
	for i := 0; i < n; i++ {
		IncIP(ip)
	}
}

func IncIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
