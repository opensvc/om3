package network

import (
	"fmt"
	"net"
	"strconv"
	"strings"

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

/*
MACFromIP4 returns a mac address using a 0a:58 prefix followed by the bridge
ipv4 address converted to hexa (same algorithm used in k8s).

When the device with the lowest mac is removed from the bridge or when
a new device with the lowest mac is added to the bridge, all containers
can experience tcp hangs while the arp table resynchronizes.

Setting a mac address to the bridge explicitely avoids these mac address
changes.
*/
func MACFromIP4(ip net.IP) (net.HardwareAddr, error) {
	mac := "0a:58"
	for _, s := range strings.Split(ip.String(), ".") {
		i, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}
		mac += fmt.Sprintf(":%.2x", i)
	}
	return net.ParseMAC(mac)
}
