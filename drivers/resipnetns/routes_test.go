//go:build linux

package resipnetns

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The default route of the namespace is set only when it has none, as v2
// does: the namespace is often shared with an ip.cni resource of the same
// container, which installs the default route of its network, and replacing
// it broke the traffic the container routes through that network.
func TestDefaultRouteToSet(t *testing.T) {
	const cni = "default via 10.22.0.1 dev eth0 \n"
	const onLink = "default dev eth3 scope link \n"

	for _, tc := range []struct {
		name     string
		defaults string
		gateway  string
		want     []string
	}{
		{
			name:     "the default route of an ip.cni of the namespace is kept",
			defaults: cni,
			want:     nil,
		},
		{
			name:     "a default route through a gateway is kept, whatever the gateway keyword",
			defaults: cni,
			gateway:  "45.13.156.1",
			want:     nil,
		},
		{
			name:     "a default route through an interface is kept",
			defaults: onLink,
			want:     nil,
		},
		{
			name:     "the gateway keyword replaces a default route through an interface",
			defaults: onLink,
			gateway:  "45.13.156.1",
			want:     []string{"default", "via", "45.13.156.1"},
		},
		{
			name: "no default route: through the interface",
			want: []string{"default", "dev", "eth7"},
		},
		{
			name:    "no default route: through the gateway",
			gateway: "45.13.156.1",
			want:    []string{"default", "via", "45.13.156.1"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, defaultRouteToSet(tc.defaults, tc.gateway, "eth7"))
		})
	}
}

// A routed_bridge network gives each node a range of its own: the namespace
// routes the whole network through the gateway of its range. A bridge or a lan
// network, whose connected route covers the segment, needs no route, and a
// gateway out of the connected route can not be routed through.
func TestNetworkRoute(t *testing.T) {
	_, backend, _ := net.ParseCIDR("10.132.0.0/16")
	addr := &net.IPNet{IP: net.ParseIP("10.132.1.114"), Mask: net.CIDRMask(22, 32)}
	assert.Equal(t,
		[]string{"10.132.0.0/16", "via", "10.132.0.1", "dev", "eth0"},
		networkRoute(backend, addr, "10.132.0.1", "eth0"))

	assert.Nil(t, networkRoute(backend, addr, "", "eth0"), "no gateway")
	assert.Nil(t, networkRoute(backend, addr, "10.132.8.1", "eth0"), "a gateway out of the connected route")

	_, lan, _ := net.ParseCIDR("10.32.0.0/24")
	public := &net.IPNet{IP: net.ParseIP("10.32.0.23"), Mask: net.CIDRMask(24, 32)}
	assert.Nil(t, networkRoute(lan, public, "10.32.0.1", "eth1"), "the connected route covers the segment")
	assert.Nil(t, networkRoute(backend, public, "10.32.0.1", "eth1"), "an address out of the segment")

	_, v6, _ := net.ParseCIDR("fd01::/112")
	addr6 := &net.IPNet{IP: net.ParseIP("fd01::4005"), Mask: net.CIDRMask(114, 128)}
	assert.Equal(t,
		[]string{"fd01::/112", "via", "fd01::4001", "dev", "eth0"},
		networkRoute(v6, addr6, "fd01::4001", "eth0"))
}
