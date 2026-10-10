package networklan

import (
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
)

func newLAN(t *testing.T, ini string) *T {
	t.Helper()
	testhelper.Setup(t)
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte(ini), 0600))
	n, err := object.NewNode()
	require.NoError(t, err)
	nw := network.NewFromNoder("san", n)
	require.NotNil(t, nw)
	return nw.(*T)
}

type fakeAddr string

func (a fakeAddr) Network() string { return "ip+net" }
func (a fakeAddr) String() string  { return string(a) }

func fakeInterfaces(t *testing.T, m map[string][]string) {
	t.Helper()
	prevI, prevA := interfaces, addrsOf
	t.Cleanup(func() { interfaces, addrsOf = prevI, prevA })
	interfaces = func() ([]net.Interface, error) {
		l := []net.Interface{{Name: "lo", Flags: net.FlagLoopback}}
		for name := range m {
			l = append(l, net.Interface{Name: name})
		}
		return l, nil
	}
	addrsOf = func(i net.Interface) ([]net.Addr, error) {
		l := make([]net.Addr, 0)
		for _, s := range m[i.Name] {
			l = append(l, fakeAddr(s))
		}
		if i.Name == "lo" {
			l = append(l, fakeAddr("fd01:2345:6789:4599::1/128"))
		}
		return l, nil
	}
}

const san = "[network#san]\ntype = lan\nnetwork = fd01:2345:6789:2902::/64\nranges = fd01:2345:6789:2902::5:0/120\n"

// Every node draws from the same ranges of the segment, and the addresses
// belong to the cluster.
func TestRanges(t *testing.T) {
	nw := newLAN(t, san)
	a, err := nw.AllocatableRange("n1")
	require.NoError(t, err)
	b, err := nw.AllocatableRange("n2")
	require.NoError(t, err)
	assert.Equal(t, a.String(), b.String())
	assert.Equal(t, "fd01:2345:6789:2902::/64", a.String())
	assert.True(t, nw.IsClusterWide())
	n, err := nw.Netmask()
	require.NoError(t, err)
	assert.Equal(t, 64, n)

	nw = newLAN(t, "[network#san]\ntype = lan\nnetwork = 192.168.10.0/24\nranges = 192.168.10.100-192.168.10.149 192.168.10.151-192.168.10.199\n")
	pools, err := nw.Pools()
	require.NoError(t, err)
	assert.Equal(t, "192.168.10.100-192.168.10.149 192.168.10.151-192.168.10.199", ipam.PoolsString(pools))
}

// The ranges are required, in the segment, and do not overlap.
func TestRangesRefused(t *testing.T) {
	for _, tc := range []struct {
		ranges string
		want   string
	}{
		{ranges: "", want: "ranges is not set"},
		{ranges: "ranges = fd01:2345:6789:2903::5:0/120\n", want: "is not in the segment"},
		{ranges: "ranges = fd01:2345:6789:2902::5:0/120 fd01:2345:6789:2902::5:10-fd01:2345:6789:2902::5:20\n", want: "overlaps"},
		{ranges: "ranges = fd01:2345:6789:2902::5:10\n", want: "is neither a subnet"},
	} {
		_, err := newLAN(t, "[network#san]\ntype = lan\nnetwork = fd01:2345:6789:2902::/64\n"+tc.ranges).Pools()
		assert.ErrorContains(t, err, tc.want, tc.ranges)
	}
}

// The interface is the one holding an address of the segment, an address of
// the ranges a service left on another interface not counting.
func TestHostDev(t *testing.T) {
	fakeInterfaces(t, map[string][]string{
		"enp2s0": {"10.29.1.11/24", "fd01:2345:6789:2901::11/64"},
		"enp3s0": {"fe80::1/64", "fd01:2345:6789:2902::11/64"},
		"eth9":   {"fd01:2345:6789:2902::5:12/64"},
	})
	dev, err := newLAN(t, san).HostDev()
	require.NoError(t, err)
	assert.Equal(t, "enp3s0", dev)

	dev, err = newLAN(t, san+"dev = bond0\n").HostDev()
	require.NoError(t, err)
	assert.Equal(t, "bond0", dev, "the keyword wins")
}

// A node with no address on the segment, or with several interfaces holding
// one, has the dev keyword say which.
func TestHostDevAmbiguous(t *testing.T) {
	fakeInterfaces(t, map[string][]string{
		"enp2s0": {"10.29.1.11/24"},
	})
	_, err := newLAN(t, san).HostDev()
	assert.ErrorContains(t, err, "set the dev keyword")

	fakeInterfaces(t, map[string][]string{
		"eth1": {"fd01:2345:6789:2902::11/64"},
		"eth2": {"fd01:2345:6789:2902::12/64"},
	})
	_, err = newLAN(t, san).HostDev()
	assert.ErrorContains(t, err, "set the dev keyword")
}

// The gateway is the router of the segment: an address of the segment, out of
// the ranges om hands out.
func TestGateway(t *testing.T) {
	gw, err := newLAN(t, san).Gateway()
	require.NoError(t, err)
	assert.Nil(t, gw, "none when not set")

	gw, err = newLAN(t, san+"gateway = fd01:2345:6789:2902::1\n").Gateway()
	require.NoError(t, err)
	assert.Equal(t, "fd01:2345:6789:2902::1", gw.String())

	_, err = newLAN(t, san+"gateway = fd01:2345:6789:2902::5:1\n").Gateway()
	assert.ErrorContains(t, err, "which om hands out")

	_, err = newLAN(t, san+"gateway = fd01:2345:6789:2903::1\n").Gateway()
	assert.ErrorContains(t, err, "is not on the segment")

	_, err = newLAN(t, san+"gateway = router\n").Gateway()
	assert.ErrorContains(t, err, "is not an ip address")
}
