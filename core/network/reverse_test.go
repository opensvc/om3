package network

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/ipam"
)

func TestRangePrefixes(t *testing.T) {
	for _, tc := range []struct {
		first, last string
		want        []string
	}{
		{"10.100.0.0", "10.100.3.255", []string{"10.100.0.0/22"}},
		{"192.168.10.100", "192.168.10.149", []string{"192.168.10.100/30", "192.168.10.104/29", "192.168.10.112/28", "192.168.10.128/28", "192.168.10.144/30", "192.168.10.148/31"}},
		{"10.0.0.7", "10.0.0.7", []string{"10.0.0.7/32"}},
		{"fd01::5:0", "fd01::5:ff", []string{"fd01::5:0/120"}},
	} {
		l := rangePrefixes(netip.MustParseAddr(tc.first), netip.MustParseAddr(tc.last))
		got := make([]string, len(l))
		for i, p := range l {
			got[i] = p.String()
		}
		assert.Equal(t, tc.want, got, tc.first+"-"+tc.last)
	}
}

func TestPrefixZones(t *testing.T) {
	for prefix, want := range map[string][]string{
		"10.100.0.0/22":     {"0.100.10.in-addr.arpa", "1.100.10.in-addr.arpa", "2.100.10.in-addr.arpa", "3.100.10.in-addr.arpa"},
		"10.22.0.0/16":      {"22.10.in-addr.arpa"},
		"10.0.0.0/8":        {"10.in-addr.arpa"},
		"10.0.0.7/32":       {"7.0.0.10.in-addr.arpa"},
		"192.168.10.144/30": {"144.10.168.192.in-addr.arpa", "145.10.168.192.in-addr.arpa", "146.10.168.192.in-addr.arpa", "147.10.168.192.in-addr.arpa"},
		"fdfe::/112":        {"0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.e.f.d.f.ip6.arpa"},
		"fd01::5:0/120":     {"0.0.5.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.1.0.d.f.ip6.arpa"},
		"fdff::/118":        {"0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.f.f.d.f.ip6.arpa", "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.f.f.d.f.ip6.arpa", "2.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.f.f.d.f.ip6.arpa", "3.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.f.f.d.f.ip6.arpa"},
	} {
		assert.Equal(t, want, prefixZones(netip.MustParsePrefix(prefix)), prefix)
	}
}

type fakePoolNetwork struct {
	Networker
	pools []ipam.Pool
}

func (t fakePoolNetwork) Type() string                { return "lan" }
func (t fakePoolNetwork) Pools() ([]ipam.Pool, error) { return t.pools, nil }
func (t fakePoolNetwork) Name() string                { return "san" }

// The zones of a network handing out some of its segment are the ones of its
// ranges, holding none of the other addresses of the segment.
func TestReverseZonesOfRanges(t *testing.T) {
	pools, err := ipam.ParsePools([]string{"fd01:2345:6789:2902::5:0/120", "192.0.2.10-192.0.2.11"})
	assert.NoError(t, err)
	zones, err := ReverseZones(fakePoolNetwork{pools: pools})
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"0.0.5.0.0.0.0.0.0.0.0.0.0.0.2.0.9.2.9.8.7.6.5.4.3.2.1.0.d.f.ip6.arpa",
		"10.2.0.192.in-addr.arpa",
		"11.2.0.192.in-addr.arpa",
	}, zones)
	assert.True(t, InReverseZones("5.0.0.0.5.0.0.0.0.0.0.0.0.0.0.0.2.0.9.2.9.8.7.6.5.4.3.2.1.0.d.f.ip6.arpa.", zones))
	assert.False(t, InReverseZones("1.1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.2.0.9.2.9.8.7.6.5.4.3.2.1.0.d.f.ip6.arpa.", zones), "a node address of the segment")
}
