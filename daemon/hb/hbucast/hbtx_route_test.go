package hbucast

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func ips(l ...string) []net.IP {
	out := make([]net.IP, len(l))
	for i, s := range l {
		out[i] = net.ParseIP(s)
	}
	return out
}

func ipNets(t *testing.T, l ...string) []*net.IPNet {
	t.Helper()
	out := make([]*net.IPNet, len(l))
	for i, s := range l {
		ip, n, err := net.ParseCIDR(s)
		if err != nil {
			t.Fatal(err)
		}
		n.IP = ip
		out[i] = n
	}
	return out
}

// The addresses of a Debian QA node whose hosts file does not name it: the
// myhostname nss module resolves its name to every address it has,
// link-local ones first. Its peer resolves to its address on the prd
// network, which is where it expects the messages of the node from.
func TestPickRoutesPrefersTheSubnetOfThePeer(t *testing.T) {
	resolved := ips(
		"fe80::2023:24ff:fe17:111",
		"fd01:2345:6789:2301::11",
		"10.23.1.11",
		"10.23.2.11",
		"10.23.0.11",
	)
	var localIPs []net.IP
	for _, ip := range resolved {
		if usableIP(ip) {
			localIPs = append(localIPs, ip)
		}
	}
	nets := ipNets(t,
		"fe80::2023:24ff:fe17:111/64",
		"fd01:2345:6789:2301::11/64",
		"10.23.1.11/24",
		"10.23.2.11/24",
		"10.23.0.11/24",
	)

	routes := pickRoutes(ips("fe80::2023:24ff:fe17:112", "10.23.0.12"), localIPs, nets, "10000")
	assert.Equal(t, []dialRoute{
		{addr: "10.23.0.12:10000", source: net.ParseIP("10.23.0.11")},
	}, routes, "the link-local peer address is not dialed, and the other is dialed from the local address of its subnet")
}

func TestPickRoutesOrder(t *testing.T) {
	localIPs := ips("10.0.1.1", "fd00::1")
	nets := ipNets(t, "10.0.1.1/24", "fd00::1/64")

	routes := pickRoutes(ips("10.0.9.2", "fd00::2", "10.0.1.2"), localIPs, nets, "1")
	assert.Equal(t, []dialRoute{
		{addr: "[fd00::2]:1", source: net.ParseIP("fd00::1")},
		{addr: "10.0.1.2:1", source: net.ParseIP("10.0.1.1")},
		{addr: "10.0.9.2:1", source: net.ParseIP("10.0.1.1")},
	}, routes, "the subnet routes first, then the others from a local address of their family")

	routes = pickRoutes(ips("fd09::2"), ips("10.0.1.1"), nets, "1")
	assert.Equal(t, []dialRoute{{addr: "[fd09::2]:1"}}, routes, "no local address of its family: the kernel picks the source")

	assert.Empty(t, pickRoutes(ips("fe80::2", "127.0.0.1"), localIPs, nets, "1"), "no usable peer address")
}
