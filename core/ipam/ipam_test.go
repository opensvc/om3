package ipam

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
)

func cidr(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, ipnet, err := net.ParseCIDR(s)
	require.NoError(t, err)
	return ipnet
}

func newT(t *testing.T, network string) *T {
	t.Helper()
	return &T{
		Name:    "backend3",
		Range:   cidr(t, network),
		Gateway: net.ParseIP("10.100.0.1"),
		Dir:     filepath.Join(t.TempDir(), "backend3"),
	}
}

func TestAllocateStaysInTheRange(t *testing.T) {
	ipam := newT(t, "10.100.0.0/24")
	for _, key := range []string{"root/svc/pod1#ip#0", "root/svc/pod2#ip#0", "test/svc/db1#ip#1"} {
		ip, err := ipam.Allocate(key)
		require.NoErrorf(t, err, "allocate %s", key)
		assert.Truef(t, ipam.Range.Contains(ip), "%s is outside %s", ip, ipam.Range)
	}
}

// TestAllocateIsStableForAKey pins that an object keeps its address across
// restarts, so its name keeps resolving to the same place.
func TestAllocateIsStableForAKey(t *testing.T) {
	ipam := newT(t, "10.100.0.0/24")
	first, err := ipam.Allocate("root/svc/pod1#ip#0")
	require.NoError(t, err)

	again, err := ipam.Allocate("root/svc/pod1#ip#0")
	require.NoError(t, err)
	assert.Equal(t, first.String(), again.String(), "a second allocation must return the address already held")

	require.NoError(t, ipam.Free("root/svc/pod1#ip#0"))
	after, err := ipam.Allocate("root/svc/pod1#ip#0")
	require.NoError(t, err)
	assert.Equal(t, first.String(), after.String(), "an address freed and drawn again must be the same one")
}

func TestAllocateHandsTheSameAddressOnce(t *testing.T) {
	ipam := newT(t, "10.100.0.0/28")
	seen := make(map[string]string)
	for _, key := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		ip, err := ipam.Allocate(key)
		require.NoErrorf(t, err, "allocate %s", key)
		if other, ok := seen[ip.String()]; ok {
			t.Fatalf("%s was given to %s and to %s", ip, other, key)
		}
		seen[ip.String()] = key
	}
}

// TestAllocateSkipsTheAddressesOfTheRangeItself pins the three an object never
// gets: the address naming the range, the broadcast address, and the gateway.
func TestAllocateSkipsTheAddressesOfTheRangeItself(t *testing.T) {
	ipam := newT(t, "10.100.0.0/29")
	ipam.Gateway = net.ParseIP("10.100.0.1")
	for i := range 5 {
		ip, err := ipam.Allocate(string(rune('a' + i)))
		require.NoError(t, err)
		assert.NotEqual(t, "10.100.0.0", ip.String(), "the address naming the range")
		assert.NotEqual(t, "10.100.0.7", ip.String(), "the broadcast address")
		assert.NotEqual(t, "10.100.0.1", ip.String(), "the gateway")
	}
}

// TestAllocateAvoidsWhatAnotherAllocatorHandedOut pins that om does not hand
// out an address the host-local plugin already gave, which it would while
// ip.cni is still served by it.
func TestAllocateAvoidsWhatAnotherAllocatorHandedOut(t *testing.T) {
	ipam := newT(t, "10.100.0.0/29")
	peer := t.TempDir()
	ipam.PeerDirs = []string{peer}
	// .0 names the range, .1 is the gateway and .7 is the broadcast address,
	// so .2 to .6 are allocatable. Leave one.
	for _, addr := range []string{"10.100.0.2", "10.100.0.3", "10.100.0.4", "10.100.0.5"} {
		require.NoError(t, os.WriteFile(filepath.Join(peer, addr), []byte("2083975\n"), 0644))
	}
	ip, err := ipam.Allocate("root/svc/pod1#ip#0")
	require.NoError(t, err)
	assert.Equal(t, "10.100.0.6", ip.String(), "the only address left")
}

// TestAllocateAvoidsWhatTheClusterReportsInUse pins the second source: the
// daemon sees the addresses of objects whose reservation file is on a node
// this one cannot read.
func TestAllocateAvoidsWhatTheClusterReportsInUse(t *testing.T) {
	ipam := newT(t, "10.100.0.0/29")
	ipam.InUse = func() ([]net.IP, error) {
		return []net.IP{
			net.ParseIP("10.100.0.2"),
			net.ParseIP("10.100.0.3"),
			net.ParseIP("10.100.0.4"),
			net.ParseIP("10.100.0.6"),
		}, nil
	}
	ip, err := ipam.Allocate("root/svc/pod1#ip#0")
	require.NoError(t, err)
	assert.Equal(t, "10.100.0.5", ip.String(), "the only address left")
}

func TestAllocateReportsAFullRange(t *testing.T) {
	// /30 holds 4 addresses: the range, the gateway, one host, the broadcast.
	ipam := newT(t, "10.100.0.0/30")
	first, err := ipam.Allocate("a")
	require.NoError(t, err)
	assert.Equal(t, "10.100.0.2", first.String())

	_, err = ipam.Allocate("b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no free address")
}

func TestFreeLeavesTheReservationOfAnotherKey(t *testing.T) {
	ipam := newT(t, "10.100.0.0/24")
	ip, err := ipam.Allocate("root/svc/pod1#ip#0")
	require.NoError(t, err)

	require.NoError(t, ipam.Free("root/svc/pod2#ip#0"))
	held, err := ipam.Allocated("root/svc/pod1#ip#0")
	require.NoError(t, err)
	assert.Equal(t, ip.String(), held.String(), "freeing another key must leave this reservation")
}

func TestFreeOfAnUnknownKeyIsNotAnError(t *testing.T) {
	ipam := newT(t, "10.100.0.0/24")
	require.NoError(t, ipam.Free("root/svc/never#ip#0"))
}

// TestAllocateInAnIPV6Range pins that a range no walk could enumerate is
// allocated in all the same, the key deciding where the walk starts.
func TestAllocateInAnIPV6Range(t *testing.T) {
	ipam := &T{
		Name:  "backend1",
		Range: cidr(t, "fdfe::/64"),
		Dir:   filepath.Join(t.TempDir(), "backend1"),
	}
	seen := make(map[string]bool)
	for _, key := range []string{"a", "b", "c"} {
		ip, err := ipam.Allocate(key)
		require.NoErrorf(t, err, "allocate %s", key)
		assert.Truef(t, ipam.Range.Contains(ip), "%s is outside %s", ip, ipam.Range)
		assert.Nil(t, ip.To4(), "%s must be an ipv6 address", ip)
		assert.Falsef(t, seen[ip.String()], "%s was handed out twice", ip)
		seen[ip.String()] = true
	}
}

// TestAllocateRefusesANetworkWithNoRange pins that a network om allocates
// nothing in, the lo network among them, says so rather than answering with
// an address of some other range.
func TestAllocateRefusesANetworkWithNoRange(t *testing.T) {
	ipam := &T{Name: "lo", Dir: t.TempDir()}
	_, err := ipam.Allocate("root/svc/pod1#ip#0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "allocates no address")
}

// TestGatewayIsTheFirstAddressPlusOne pins the rule both network drivers use
// to place their bridge, which is the address the allocator must not hand out.
func TestGatewayIsTheFirstAddressPlusOne(t *testing.T) {
	for _, tc := range []struct{ network, want string }{
		{"10.100.0.0/24", "10.100.0.1"},
		{"10.100.1.0/24", "10.100.1.1"},
		{"10.22.0.0/16", "10.22.0.1"},
		{"fdfe::/114", "fdfe::1"},
		{"fdfe::4000/114", "fdfe::4001"},
	} {
		assert.Equal(t, tc.want, Gateway(cidr(t, tc.network)).String(), tc.network)
	}
	assert.Nil(t, Gateway(nil))
}

// TestAllocateGivesEachResourceItsOwnAddress pins the case an instance with
// several ip resources in one network needs. om puts no limit on how many an
// object holds, nor on how many of them share a network, so the key names the
// resource: keying on the object alone would have ip#0 and ip#1 draw the same
// address.
func TestAllocateGivesEachResourceItsOwnAddress(t *testing.T) {
	ipam := newT(t, "10.100.0.0/24")
	p1, err := naming.ParsePath("root/svc/pod1")
	require.NoError(t, err)
	p2, err := naming.ParsePath("root/svc/pod2")
	require.NoError(t, err)

	keys := []string{
		Key(p1, "ip#0"), Key(p1, "ip#1"), Key(p1, "ip#2"),
		Key(p2, "ip#0"), Key(p2, "ip#1"),
	}
	held := make(map[string]string, len(keys))
	for _, key := range keys {
		ip, err := ipam.Allocate(key)
		require.NoErrorf(t, err, "allocate %s", key)
		if other, ok := held[ip.String()]; ok {
			t.Fatalf("%s was given to %s and to %s", ip, other, key)
		}
		held[ip.String()] = key
	}
	assert.Len(t, held, len(keys))

	// And each keeps its own across a restart of the instance.
	for addr, key := range held {
		again, err := ipam.Allocate(key)
		require.NoError(t, err)
		assert.Equalf(t, addr, again.String(), "%s must keep its address", key)
	}
}

// TestAllocateResolvesTheCollisionsOfASmallRange pins that the walk finds the
// free addresses of a range too small for the hashes to spread over. Filling a
// range is where two keys landing on one candidate stops being unlikely.
func TestAllocateResolvesTheCollisionsOfASmallRange(t *testing.T) {
	// .0 names the range, .1 is the gateway, .15 is the broadcast address:
	// 13 addresses for 13 resources.
	ipam := newT(t, "10.100.0.0/28")
	p, err := naming.ParsePath("root/svc/pod1")
	require.NoError(t, err)

	held := make(map[string]string)
	for i := range 13 {
		key := Key(p, fmt.Sprintf("ip#%d", i))
		ip, err := ipam.Allocate(key)
		require.NoErrorf(t, err, "allocate %s of 13 in a /28", key)
		if other, ok := held[ip.String()]; ok {
			t.Fatalf("%s was given to %s and to %s", ip, other, key)
		}
		held[ip.String()] = key
	}
	assert.Len(t, held, 13, "every allocatable address of the range")

	_, err = ipam.Allocate(Key(p, "ip#13"))
	require.Error(t, err, "the range is full")
}

// TestAdoptKeepsTheAddressAResourceHolds pins the move from another
// allocator: a resource that already has an address keeps it, rather than
// being handed a new one and renumbered.
func TestAdoptKeepsTheAddressAResourceHolds(t *testing.T) {
	ipam := newT(t, "10.100.0.0/24")
	p, err := naming.ParsePath("root/svc/pod1")
	require.NoError(t, err)
	key := Key(p, "ip#0")

	_, err = ipam.Adopt([]Reservation{{IP: net.ParseIP("10.100.0.22"), Key: key}})
	require.NoError(t, err)

	ip, err := ipam.Allocate(key)
	require.NoError(t, err)
	assert.Equal(t, "10.100.0.22", ip.String(), "the address the resource already holds")
}

// TestAdoptDoesNotHandOutAnAdoptedAddress pins the other half: an address
// recorded for one resource is not drawn by another.
func TestAdoptDoesNotHandOutAnAdoptedAddress(t *testing.T) {
	// .0 names the range, .1 is the gateway, .7 is the broadcast address.
	ipam := newT(t, "10.100.0.0/29")
	adopted := []Reservation{
		{IP: net.ParseIP("10.100.0.2"), Key: "a"},
		{IP: net.ParseIP("10.100.0.3"), Key: "b"},
		{IP: net.ParseIP("10.100.0.4"), Key: "c"},
		{IP: net.ParseIP("10.100.0.5"), Key: "d"},
	}
	n, err := ipam.Adopt(adopted)
	require.NoError(t, err)
	assert.Equal(t, 4, n, "four addresses recorded")

	ip, err := ipam.Allocate("e")
	require.NoError(t, err)
	assert.Equal(t, "10.100.0.6", ip.String(), "the only address left")
}

// TestAdoptIsIdempotentAndLate pins that an adoption run twice changes
// nothing, and that one run after an allocation does not take an address from
// the resource that drew it.
func TestAdoptIsIdempotentAndLate(t *testing.T) {
	ipam := newT(t, "10.100.0.0/24")
	drawn, err := ipam.Allocate("first")
	require.NoError(t, err)

	adopted := []Reservation{{IP: drawn, Key: "late"}}
	n, err := ipam.Adopt(adopted)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "the address is already reserved by the resource that drew it")
	_, err = ipam.Adopt(adopted)
	require.NoError(t, err)

	held, err := ipam.Allocated("first")
	require.NoError(t, err)
	assert.Equal(t, drawn.String(), held.String(), "the resource that drew it keeps it")

	other, err := ipam.Allocate("late")
	require.NoError(t, err)
	assert.NotEqual(t, drawn.String(), other.String(), "the late adoption gets an address of its own")
}

// TestAdoptSkipsWhatIsNotThisNodeRange pins that an address of another node,
// which the cluster status reports alongside this node's, is not recorded
// here.
func TestAdoptSkipsWhatIsNotThisNodeRange(t *testing.T) {
	ipam := newT(t, "10.100.0.0/24")
	n, err := ipam.Adopt([]Reservation{
		{IP: net.ParseIP("10.100.1.22"), Key: "on another node"},
		{IP: nil, Key: "no address"},
		{IP: net.ParseIP("10.100.0.22"), Key: ""},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	entries, err := os.ReadDir(ipam.Dir)
	if !os.IsNotExist(err) {
		require.NoError(t, err)
		assert.Empty(t, entries, "nothing of another range is recorded")
	}
}
