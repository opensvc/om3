package ipam

import (
	"math/big"
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

// A reservation the ranges no longer hold, as after they were narrowed, is
// given up for an address they hold.
func TestAllocateOutOfNarrowedPools(t *testing.T) {
	_, segment, _ := net.ParseCIDR("192.168.10.0/24")
	dir := t.TempDir()
	wide, err := ParsePools([]string{"192.168.10.100-192.168.10.199"})
	require.NoError(t, err)
	i := &T{Name: "lan1", Range: segment, Pools: wide, Dir: dir}
	before, err := i.Allocate("a")
	require.NoError(t, err)

	others := make([]string, 0)
	for _, p := range wide {
		if !p.Contains(before) {
			continue
		}
		if !before.Equal(p.First) {
			others = append(others, p.First.String()+"-"+prevIP(before).String())
		}
		if !before.Equal(p.Last) {
			others = append(others, nextIP(before).String()+"-"+p.Last.String())
		}
	}
	narrowed, err := ParsePools(others)
	require.NoError(t, err)
	i = &T{Name: "lan1", Range: segment, Pools: narrowed, Dir: dir}
	after, err := i.Allocate("a")
	require.NoError(t, err)
	assert.False(t, after.Equal(before))
	assert.True(t, i.Contains(after))
	held, err := i.Allocated("a")
	require.NoError(t, err)
	assert.True(t, held.Equal(after), "the address out of the ranges is released")
}

func prevIP(ip net.IP) net.IP {
	return intToIP(new(big.Int).Sub(ipToInt(ip), big.NewInt(1)), ip.To4() != nil)
}

func nextIP(ip net.IP) net.IP {
	return intToIP(new(big.Int).Add(ipToInt(ip), big.NewInt(1)), ip.To4() != nil)
}

// A reservation the ranges no longer hold is kept when they hold no free
// address to replace it: the address is still configured, and no other
// resource may draw it.
func TestAllocateOutOfNarrowedFullPools(t *testing.T) {
	_, segment, _ := net.ParseCIDR("192.168.10.0/24")
	dir := t.TempDir()
	wide, err := ParsePools([]string{"192.168.10.100-192.168.10.101"})
	require.NoError(t, err)
	i := &T{Name: "lan1", Range: segment, Pools: wide, Dir: dir}
	a, err := i.Allocate("a")
	require.NoError(t, err)
	b, err := i.Allocate("b")
	require.NoError(t, err)

	narrowed, err := ParsePools([]string{b.String() + "-" + b.String()})
	require.NoError(t, err)
	i = &T{Name: "lan1", Range: segment, Pools: narrowed, Dir: dir}
	_, err = i.Allocate("a")
	assert.Error(t, err, "no free address")
	held, err := i.Allocated("a")
	require.NoError(t, err)
	assert.True(t, held.Equal(a), "the address out of the ranges stays reserved")

	require.NoError(t, i.Free("b"))
	got, err := i.Allocate("a")
	require.NoError(t, err)
	assert.True(t, got.Equal(b))
	reservations, err := i.Reservations()
	require.NoError(t, err)
	assert.Len(t, reservations, 1, "the address out of the ranges is released once replaced")
}

// A key holding an address in the ranges gives up the ones it holds out of
// them, and a key holding only an address out of them keeps it.
func TestDropReplaced(t *testing.T) {
	_, segment, _ := net.ParseCIDR("192.168.10.0/24")
	pools, err := ParsePools([]string{"192.168.10.100-192.168.10.199"})
	require.NoError(t, err)
	i := &T{Name: "lan1", Range: segment, Pools: pools, Dir: t.TempDir()}
	for addr, key := range map[string]string{
		"192.168.10.10":  "a",
		"192.168.10.110": "a",
		"192.168.10.20":  "b",
	} {
		ok, err := i.reserve(net.ParseIP(addr), key)
		require.NoError(t, err)
		require.True(t, ok)
	}
	n, err := i.DropReplaced()
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	got := make(map[string]string)
	reservations, err := i.Reservations()
	require.NoError(t, err)
	for _, r := range reservations {
		got[r.IP.String()] = r.Key
	}
	assert.Equal(t, map[string]string{"192.168.10.110": "a", "192.168.10.20": "b"}, got)
}

// An address the ranges no longer hold, still configured, is reserved again
// for the key that could not give it up, and the replacement it drew is
// released. An address another key holds is refused.
func TestKeep(t *testing.T) {
	_, segment, _ := net.ParseCIDR("192.168.10.0/24")
	pools, err := ParsePools([]string{"192.168.10.100-192.168.10.199"})
	require.NoError(t, err)
	i := &T{Name: "lan1", Range: segment, Pools: pools, Dir: t.TempDir()}
	_, err = i.Allocate("a")
	require.NoError(t, err)
	previous := net.ParseIP("192.168.10.10")

	require.NoError(t, i.Keep(previous, "a"))
	held, err := i.Allocated("a")
	require.NoError(t, err)
	assert.True(t, held.Equal(previous))
	reservations, err := i.Reservations()
	require.NoError(t, err)
	assert.Len(t, reservations, 1, "the address drawn is released")
	require.NoError(t, i.Keep(previous, "a"), "kept twice")

	_, err = i.Allocate("b")
	require.NoError(t, err)
	b, err := i.Allocated("b")
	require.NoError(t, err)
	assert.Error(t, i.Keep(b, "a"))
}
