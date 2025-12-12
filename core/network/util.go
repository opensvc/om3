package network

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

func sectionName(networkName string) string {
	return "network#" + networkName
}

func cKey(networkName string, option string) key.T {
	section := sectionName(networkName)
	return key.New(section, option)
}

func cString(config *xconfig.T, networkName string, option string) string {
	k := cKey(networkName, option)
	return config.GetString(k)
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

Setting a mac address to the bridge explicitly avoids these mac address
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

// IsDisabled returns true if the network keyword value is "none"
func IsDisabled(t Networker) bool {
	s := t.Network()
	if strings.ToLower(s) == "none" {
		return true
	}
	return false
}

// IsValid returns true if the network configuration is sane enough to setup.
func IsValid(t Networker) bool {
	s := t.Network()
	if s == "" && t.AllowEmptyNetwork() {
		return true
	}
	if _, _, err := net.ParseCIDR(s); err != nil {
		return false
	}
	return true
}

func IPReachableFrom(peerIP net.IP) (net.IP, *net.IPNet, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, nil, err
	}
	for _, addr := range addrs {
		s := addr.String()
		ip, ipnet, err := net.ParseCIDR(s)
		if err != nil {
			continue
		}
		if !ipnet.Contains(peerIP) {
			continue
		}
		return ip, ipnet, nil
	}
	return nil, nil, nil
}

func IsSameNetwork(localIP, peerIP net.IP) (bool, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false, err
	}
	for _, addr := range addrs {
		if ip, ipnet, _ := net.ParseCIDR(addr.String()); ip.Equal(localIP) {
			return ipnet.Contains(peerIP), nil
		}
	}
	return false, nil
}
