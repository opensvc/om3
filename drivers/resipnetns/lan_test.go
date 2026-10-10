//go:build linux

package resipnetns

import (
	"net"
	"testing"

	"github.com/golang-collections/collections/set"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/network"
)

// fakeLan is a lan network whose node interface on the segment is eth0, on a
// /64 segment.
type fakeLan struct {
	network.Networker
}

func (fakeLan) Name() string             { return "san" }
func (fakeLan) HostDev() (string, error) { return "eth0", nil }
func (fakeLan) Netmask() (int, error)    { return 64, nil }

// On a lan network, the link of the namespace is a child of the interface of
// the node on the segment, with the prefix length of the segment, and no
// gateway is taken from the range. The modes that would not put the address
// on the segment, or would take the interface of the node away, are refused.
func TestConfigureLan(t *testing.T) {
	for _, tc := range []struct {
		name        string
		mode        string
		dev         string
		wantDev     string
		wantNetmask string
		wantErr     bool
	}{
		{name: "macvlan is a child of the node interface", mode: "macvlan", wantDev: "eth0", wantNetmask: "64"},
		{name: "ipvlan-l2 is a child of the node interface", mode: "ipvlan-l2", wantDev: "eth0", wantNetmask: "64"},
		{name: "bridge plugs into the node interface", mode: "bridge", wantDev: "eth0", wantNetmask: "64"},
		{name: "an explicit dev wins", mode: "macvlan", dev: "eth1", wantDev: "eth1", wantNetmask: "64"},
		{name: "ipvlan-l3 is refused", mode: "ipvlan-l3", wantErr: true},
		{name: "ipvlan-l3s is refused", mode: "ipvlan-l3s", wantErr: true},
		{name: "dedicated without dev is refused", mode: "dedicated", wantErr: true},
		{name: "dedicated on the node interface is refused", mode: "dedicated", dev: "eth0", wantErr: true},
		{name: "dedicated on an interface of its own", mode: "dedicated", dev: "eth1", wantDev: "eth1", wantNetmask: "64"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &T{Mode: tc.mode, Dev: tc.dev}
			r.Tags = set.New()
			nw := fakeLan{}
			r.configureLan(nw, nw)
			if tc.wantErr {
				assert.Error(t, r.netErr)
				return
			}
			assert.NoError(t, r.netErr)
			assert.Equal(t, tc.wantDev, r.Dev)
			assert.Equal(t, tc.wantNetmask, r.Netmask)
			assert.Empty(t, r.Gateway)
		})
	}
}

// fakeLanRouted is a lan network naming the router of its segment.
type fakeLanRouted struct {
	fakeLan
}

func (fakeLanRouted) Gateway() (net.IP, error) { return net.ParseIP("fd01::1"), nil }

// The gateway of a lan network is the router it names, unless the resource
// names its own, which then prevails to set the default route.
func TestConfigureLanGateway(t *testing.T) {
	nw := fakeLanRouted{}
	r := &T{Mode: "macvlan"}
	r.Tags = set.New()
	r.configureLan(nw, nw)
	require.NoError(t, r.netErr)
	assert.Equal(t, "fd01::1", r.Gateway)
	assert.Equal(t, gatewayOfNetwork, r.gatewayRank)

	r = &T{Mode: "macvlan", Gateway: "fd01::fe", gatewayRank: gatewayOwn}
	r.Tags = set.New()
	r.configureLan(nw, nw)
	require.NoError(t, r.netErr)
	assert.Equal(t, "fd01::fe", r.Gateway)
	assert.Equal(t, gatewayOwn, r.gatewayRank)
}
