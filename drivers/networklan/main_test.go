package networklan

import (
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// Every node draws from the whole range, and the range belongs to the cluster.
func TestRange(t *testing.T) {
	nw := newLAN(t, "[network#san]\ntype = lan\nnetwork = fd01:2345:6789:4599::1:0/112\nnetmask = 64\n")
	a, err := nw.AllocatableRange("n1")
	require.NoError(t, err)
	b, err := nw.AllocatableRange("n2")
	require.NoError(t, err)
	assert.Equal(t, a.String(), b.String())
	assert.Equal(t, "fd01:2345:6789:4599::1:0/112", a.String())
	assert.True(t, nw.IsClusterWide())
}

const sanRange = "[network#san]\ntype = lan\nnetwork = fd01:2345:6789:2902::5:0/120\n"

// The interface and the prefix length come from the address of the node whose
// prefix holds the range.
func TestFromTheNodeAddress(t *testing.T) {
	fakeInterfaces(t, map[string][]string{
		"enp2s0": {"10.29.1.11/24", "fd01:2345:6789:2901::11/64"},
		"enp3s0": {"fe80::1/64", "fd01:2345:6789:2902::11/64"},
	})
	nw := newLAN(t, sanRange)
	dev, err := nw.HostDev()
	require.NoError(t, err)
	assert.Equal(t, "enp3s0", dev)
	n, err := nw.Netmask()
	require.NoError(t, err)
	assert.Equal(t, 64, n)
}

// The keywords override what the node address says.
func TestKeywordsOverride(t *testing.T) {
	fakeInterfaces(t, map[string][]string{
		"enp3s0": {"fd01:2345:6789:2902::11/64"},
		"bond0":  {"fd01:2345:6789:2902::21/56"},
	})
	nw := newLAN(t, sanRange+"netmask = 96\n")
	n, err := nw.Netmask()
	require.NoError(t, err)
	assert.Equal(t, 96, n)
	dev, err := nw.HostDev()
	require.NoError(t, err)
	assert.Equal(t, "enp3s0", dev, "the deepest prefix wins")

	nw = newLAN(t, sanRange+"dev = bond0\n")
	n, err = nw.Netmask()
	require.NoError(t, err)
	assert.Equal(t, 56, n, "the address of the interface dev names")

	_, err = newLAN(t, sanRange+"netmask = 121\n").Netmask()
	assert.ErrorContains(t, err, "the segment must hold the range")
}

// A node with no address on the segment has the keywords say it all.
func TestNoNodeAddress(t *testing.T) {
	fakeInterfaces(t, map[string][]string{
		"enp2s0": {"10.29.1.11/24"},
		"vlan9":  {"fe80::1/64"},
	})
	_, err := newLAN(t, sanRange).HostDev()
	assert.ErrorContains(t, err, "set the dev and netmask keywords")

	nw := newLAN(t, sanRange+"dev = vlan9\n")
	dev, err := nw.HostDev()
	require.NoError(t, err)
	assert.Equal(t, "vlan9", dev)
	_, err = nw.Netmask()
	assert.ErrorContains(t, err, "set the netmask keyword")

	nw = newLAN(t, sanRange+"dev = vlan9\nnetmask = 64\n")
	n, err := nw.Netmask()
	require.NoError(t, err)
	assert.Equal(t, 64, n)
}

// Two interfaces with an address of an equally deep prefix holding the range
// leave the interface to the dev keyword.
func TestTwoInterfacesOnTheSegment(t *testing.T) {
	fakeInterfaces(t, map[string][]string{
		"eth1": {"fd01:2345:6789:2902::11/64"},
		"eth2": {"fd01:2345:6789:2902::12/64"},
	})
	_, err := newLAN(t, sanRange).HostDev()
	assert.ErrorContains(t, err, "set the dev keyword")
}
