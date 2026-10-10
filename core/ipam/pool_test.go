package ipam

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePool(t *testing.T) {
	for _, tc := range []struct {
		s       string
		want    string
		size    int64
		wantErr bool
	}{
		{s: "192.168.10.128/26", want: "192.168.10.128/26", size: 64},
		{s: "192.168.10.100-192.168.10.199", want: "192.168.10.100-192.168.10.199", size: 100},
		{s: "192.168.10.128-192.168.10.191", want: "192.168.10.128/26", size: 64},
		{s: "192.168.10.150-192.168.10.150", want: "192.168.10.150/32", size: 1},
		{s: "fd01::5:0-fd01::5:ff", want: "fd01::5:0/120", size: 256},
		{s: "fd01::5:10-fd01::5:1f", want: "fd01::5:10/124", size: 16},
		{s: "fd01::5:10-fd01::5:20", want: "fd01::5:10-fd01::5:20", size: 17},
		{s: "192.168.10.100", wantErr: true},
		{s: "192.168.10.100-10", wantErr: true},
		{s: "192.168.10.199-192.168.10.100", wantErr: true},
		{s: "192.168.10.100-fd01::1", wantErr: true},
		{s: "192.168.10.100/33", wantErr: true},
	} {
		t.Run(tc.s, func(t *testing.T) {
			p, err := ParsePool(tc.s)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, p.String())
			assert.Equal(t, tc.size, p.Size().Int64())
		})
	}
}

func TestParsePoolsRefusesOverlaps(t *testing.T) {
	_, err := ParsePools([]string{"192.168.10.100-192.168.10.149", "192.168.10.151-192.168.10.199"})
	assert.NoError(t, err)
	_, err = ParsePools([]string{"192.168.10.100-192.168.10.150", "192.168.10.128/26"})
	assert.Error(t, err)
	_, err = ParsePools([]string{"192.168.10.128/26", "192.168.10.140-192.168.10.141"})
	assert.Error(t, err)
}

// An allocator draws from its pools only, never the address naming the
// subnet nor its broadcast address, which a pool reaching the edges of the
// subnet holds, and draws them all.
func TestAllocateFromPools(t *testing.T) {
	_, segment, _ := net.ParseCIDR("192.168.10.0/24")
	pools, err := ParsePools([]string{"192.168.10.0-192.168.10.2", "192.168.10.150", "192.168.10.253-192.168.10.255"})
	require.Error(t, err, "a pool is a subnet or a span")
	pools, err = ParsePools([]string{"192.168.10.0-192.168.10.2", "192.168.10.150/32", "192.168.10.253-192.168.10.255"})
	require.NoError(t, err)
	i := &T{Name: "lan1", Range: segment, Pools: pools, Dir: t.TempDir()}

	got := make(map[string]bool)
	for _, key := range []string{"a", "b", "c", "d", "e"} {
		ip, err := i.Allocate(key)
		require.NoError(t, err)
		got[ip.String()] = true
	}
	assert.Equal(t, map[string]bool{
		"192.168.10.1":   true,
		"192.168.10.2":   true,
		"192.168.10.150": true,
		"192.168.10.253": true,
		"192.168.10.254": true,
	}, got)
	_, err = i.Allocate("f")
	assert.Error(t, err, "the pools are exhausted")

	assert.True(t, i.Contains(net.ParseIP("192.168.10.0")))
	assert.False(t, i.Contains(net.ParseIP("192.168.10.3")))
	assert.Equal(t, "192.168.10.0-192.168.10.2 192.168.10.150/32 192.168.10.253-192.168.10.255", i.PoolsString())
}
