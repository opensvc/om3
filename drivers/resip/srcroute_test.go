//go:build linux

package resip

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

func cidr(t *testing.T, s string) *net.IPNet {
	t.Helper()
	ip, ipnet, err := net.ParseCIDR(s)
	require.NoError(t, err)
	ipnet.IP = ip
	return ipnet
}

func prefix(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, ipnet, err := net.ParseCIDR(s)
	require.NoError(t, err)
	return ipnet
}

func routeStrings(l []netlink.Route) []string {
	s := make([]string, len(l))
	for i, r := range l {
		s[i] = routeString(r)
	}
	return s
}

// Each address has a table of its own, two addresses sharing a card
// included, clear of the tables the kernel names.
func TestTheTableOfAnAddressIsItsOwn(t *testing.T) {
	a, b := TableID(net.ParseIP("10.100.0.164")), TableID(net.ParseIP("10.100.0.165"))
	assert.Equal(t, 174325924, a)
	assert.NotEqual(t, a, b)
	assert.Greater(t, a, 255)
	v6 := TableID(net.ParseIP("fdfe::12"))
	assert.NotEqual(t, v6, TableID(net.ParseIP("fdfe::13")))
	assert.Greater(t, v6, 255)
	assert.Equal(t, v6, TableID(net.ParseIP("fdfe::12")), "found again from the address")
	assert.False(t, SourceRoute{Addr: cidr(t, "0.0.0.254/23")}.hasTable(), "no table of the kernel, as the main one")
	assert.True(t, SourceRoute{Addr: cidr(t, "128.0.0.254/24")}.hasTable())
	assert.NotEqual(t, TableID(net.ParseIP("0.0.0.254")), TableID(net.ParseIP("128.0.0.254")))
}

func TestTheSourceOfTheRuleIsTheAddressAlone(t *testing.T) {
	assert.Equal(t, "10.100.0.164/32", SourceRoute{Addr: cidr(t, "10.100.0.164/24")}.Src().String())
	assert.Equal(t, "fdfe::12/128", SourceRoute{Addr: cidr(t, "fdfe::12/114")}.Src().String())
}

// The table of an address holds its connected route, the route to its
// network when the connected route covers a part of it only, and the
// default route by its gateway.
func TestTheTableOfAnAddress(t *testing.T) {
	// routed_bridge: the node range of a network.
	r := SourceRoute{Addr: cidr(t, "10.100.0.164/24"), Network: prefix(t, "10.100.0.0/22"), Gateway: net.ParseIP("10.100.0.1")}
	assert.Equal(t, []string{
		"10.100.0.0/24 src 10.100.0.164",
		"10.100.0.0/22 via 10.100.0.1",
		"default via 10.100.0.1",
	}, routeStrings(r.Routes(10002, 2)))
	for _, route := range r.Routes(10002, 2) {
		assert.Equal(t, 10002, route.Table)
		assert.Equal(t, 2, route.LinkIndex)
	}

	// bridge: the whole network is connected.
	r = SourceRoute{Addr: cidr(t, "10.22.56.240/16"), Network: prefix(t, "10.22.0.0/16"), Gateway: net.ParseIP("10.22.0.1")}
	assert.Equal(t, []string{"10.22.0.0/16 src 10.22.56.240", "default via 10.22.0.1"}, routeStrings(r.Routes(10003, 3)))

	// lan with no gateway known: the lookup falls through to the main table.
	r = SourceRoute{Addr: cidr(t, "192.168.1.10/24")}
	assert.Equal(t, []string{"192.168.1.0/24 src 192.168.1.10"}, routeStrings(r.Routes(10004, 4)))

	// connected route removed: the gateway is on the link.
	r = SourceRoute{Addr: cidr(t, "192.168.1.10/32"), Gateway: net.ParseIP("192.168.1.1"), OnLink: true}
	assert.Equal(t, []string{"default via 192.168.1.1 onlink"}, routeStrings(r.Routes(10004, 4)))

	// a gateway out of the connected route is not reachable: no default.
	r = SourceRoute{Addr: cidr(t, "192.168.1.10/24"), Gateway: net.ParseIP("10.0.0.1")}
	assert.Equal(t, []string{"192.168.1.0/24 src 192.168.1.10"}, routeStrings(r.Routes(10004, 4)))

	// ipv6 routed_bridge.
	r = SourceRoute{Addr: cidr(t, "fdfe::12/114"), Network: prefix(t, "fdfe::/112"), Gateway: net.ParseIP("fdfe::1")}
	assert.Equal(t, []string{"fdfe::/114 src fdfe::12", "fdfe::/112 via fdfe::1", "default via fdfe::1"}, routeStrings(r.Routes(10005, 5)))
}

func TestNetworkRouteNeeded(t *testing.T) {
	gw := net.ParseIP("10.100.0.1")
	assert.True(t, NetworkRouteNeeded(prefix(t, "10.100.0.0/22"), prefix(t, "10.100.0.0/24"), gw))
	assert.False(t, NetworkRouteNeeded(prefix(t, "10.100.0.0/24"), prefix(t, "10.100.0.0/24"), gw), "connected whole")
	assert.False(t, NetworkRouteNeeded(nil, prefix(t, "10.100.0.0/24"), gw), "no network")
	assert.False(t, NetworkRouteNeeded(prefix(t, "10.100.0.0/22"), prefix(t, "10.100.1.0/24"), gw), "gateway out of the connected route")
}

// A rule of the address to its table is kept, a rule to another table, as
// left by a former card, is deleted.
func TestReconcileTheRulesOfASource(t *testing.T) {
	src := prefix(t, "10.100.0.164/32")
	other := prefix(t, "10.22.56.240/32")

	del, add := reconcile(nil, src, 10002)
	assert.Empty(t, del)
	assert.True(t, add)

	del, add = reconcile([]netlink.Rule{{Src: src, Table: 10002, Priority: SourceRulePriority}, {Src: other, Table: 10003, Priority: SourceRulePriority}}, src, 10002)
	assert.Empty(t, del)
	assert.False(t, add)

	del, add = reconcile([]netlink.Rule{{Src: src, Table: 10007, Priority: SourceRulePriority}, {Src: nil, Table: 254}}, src, 10002)
	require.Len(t, del, 1)
	assert.Equal(t, 10007, del[0].Table)
	assert.True(t, add)

	// A rule of the source of another priority, as one of an ip.rule
	// resource, is no rule of om's: left alone.
	del, add = reconcile([]netlink.Rule{{Src: src, Table: 100, Priority: 100}}, src, 10002)
	assert.Empty(t, del)
	assert.True(t, add)
}
