package network

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/ipam"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

func fakeClusterAddrs(t *testing.T, m map[string][]net.IP, err error) *int {
	t.Helper()
	calls := new(int)
	prev := clusterAddrs
	t.Cleanup(func() { clusterAddrs = prev })
	clusterAddrs = func(context.Context, string) (map[string][]net.IP, error) {
		*calls++
		return m, err
	}
	prevLock := lockNetwork
	t.Cleanup(func() { lockNetwork = prevLock })
	lockNetwork = func(ctx context.Context, _, _ string) (context.Context, func(), error) {
		return ctx, func() {}, nil
	}
	return calls
}

func newClusterWide(t *testing.T) *ipam.T {
	_, rng, err := net.ParseCIDR("fd01:2345:6789:4599::1:0/120")
	require.NoError(t, err)
	return &ipam.T{Name: "san", Range: rng, Dir: t.TempDir(), ClusterWide: true}
}

// A resource takes on this node the address it holds on another one, which a
// failover object moving takes along.
func TestAllocateForTakesTheAddressHeldElsewhere(t *testing.T) {
	testhelper.Setup(t)
	p, _ := naming.ParsePath("ns1/svc/san1")
	held := net.ParseIP("fd01:2345:6789:4599::1:42")
	fakeClusterAddrs(t, map[string][]net.IP{ipam.Key(p, "ip#1"): {held}}, nil)

	ip, err := AllocateFor(context.Background(), newClusterWide(t), p, "ip#1")
	require.NoError(t, err)
	assert.Equal(t, held.String(), ip.String())
}

// The address another resource holds anywhere in the cluster is not drawn,
// though this node has no reservation of it.
func TestAllocateForKeepsClearOfTheAddressesOfTheCluster(t *testing.T) {
	testhelper.Setup(t)
	p, _ := naming.ParsePath("ns1/svc/san1")
	first, err := newClusterWide(t).Allocate(ipam.Key(p, "ip#1"))
	require.NoError(t, err)

	fakeClusterAddrs(t, map[string][]net.IP{"ns1/svc/other!ip#1": {first}}, nil)
	ip, err := AllocateFor(context.Background(), newClusterWide(t), p, "ip#1")
	require.NoError(t, err)
	assert.NotEqual(t, first.String(), ip.String(), "the address the hash leads to is held by another resource")
}

func TestAllocateForRefusesTwoAddressesOfOneResource(t *testing.T) {
	testhelper.Setup(t)
	p, _ := naming.ParsePath("ns1/svc/san1")
	fakeClusterAddrs(t, map[string][]net.IP{ipam.Key(p, "ip#1"): {
		net.ParseIP("fd01:2345:6789:4599::1:42"),
		net.ParseIP("fd01:2345:6789:4599::1:43"),
	}}, nil)
	_, err := AllocateFor(context.Background(), newClusterWide(t), p, "ip#1")
	assert.ErrorContains(t, err, "holds both")
}

// Without the daemon the addresses of the other nodes are unknown, and an
// allocation that may hand out one of them fails.
func TestAllocateForNeedsTheCluster(t *testing.T) {
	testhelper.Setup(t)
	p, _ := naming.ParsePath("ns1/svc/san1")
	fakeClusterAddrs(t, nil, errors.New("connection refused"))
	_, err := AllocateFor(context.Background(), newClusterWide(t), p, "ip#1")
	assert.ErrorContains(t, err, "read the addresses the cluster holds")
}

// A network whose range is the node's own asks the cluster nothing.
func TestAllocateForNodeRange(t *testing.T) {
	testhelper.Setup(t)
	p, _ := naming.ParsePath("ns1/svc/san1")
	calls := fakeClusterAddrs(t, nil, errors.New("not to be called"))
	i := newClusterWide(t)
	i.ClusterWide = false
	_, err := AllocateFor(context.Background(), i, p, "ip#1")
	require.NoError(t, err)
	assert.Zero(t, *calls)
}

// A draw from a cluster-wide network holds the network lock while it reads
// the cluster and reserves, and a draw from a node range takes none.
func TestAllocateForLocksTheClusterWideNetwork(t *testing.T) {
	testhelper.Setup(t)
	p, _ := naming.ParsePath("ns1/svc/san1")
	fakeClusterAddrs(t, nil, nil)
	locked, released := 0, 0
	lockNetwork = func(ctx context.Context, name, key string) (context.Context, func(), error) {
		assert.Equal(t, "san", name)
		assert.Equal(t, ipam.Key(p, "ip#1"), key)
		locked++
		return ctx, func() { released++ }, nil
	}
	_, err := AllocateFor(context.Background(), newClusterWide(t), p, "ip#1")
	require.NoError(t, err)
	assert.Equal(t, 1, locked)
	assert.Equal(t, 1, released)

	i := newClusterWide(t)
	i.ClusterWide = false
	_, err = AllocateFor(context.Background(), i, p, "ip#2")
	require.NoError(t, err)
	assert.Equal(t, 1, locked, "a node range is drawn from by this node alone")
}

// A network lock not granted is an allocation not made.
func TestAllocateForNeedsTheNetworkLock(t *testing.T) {
	testhelper.Setup(t)
	p, _ := naming.ParsePath("ns1/svc/san1")
	fakeClusterAddrs(t, nil, nil)
	lockNetwork = func(context.Context, string, string) (context.Context, func(), error) {
		return nil, nil, errors.New("cluster lock held")
	}
	_, err := AllocateFor(context.Background(), newClusterWide(t), p, "ip#1")
	assert.ErrorContains(t, err, "cluster lock held")
}

// A redraw gives the address the resource held up: it is neither the one
// drawn again, nor the one taken back from another node.
func TestRedrawForDrawsAnotherAddress(t *testing.T) {
	testhelper.Setup(t)
	p, _ := naming.ParsePath("ns1/svc/san1")
	key := ipam.Key(p, "ip#1")
	i := newClusterWide(t)
	fakeClusterAddrs(t, nil, nil)
	previous, err := AllocateFor(context.Background(), i, p, "ip#1")
	require.NoError(t, err)

	// The node the resource ran on before still reports it.
	fakeClusterAddrs(t, map[string][]net.IP{key: {previous}}, nil)
	ip, err := RedrawFor(context.Background(), i, p, "ip#1", previous)
	require.NoError(t, err)
	assert.NotEqual(t, previous.String(), ip.String())
	held, err := i.Allocated(key)
	require.NoError(t, err)
	assert.Equal(t, ip.String(), held.String())
}

// A redraw from a node range gives the address up too.
func TestRedrawForNodeRange(t *testing.T) {
	testhelper.Setup(t)
	p, _ := naming.ParsePath("ns1/svc/san1")
	i := newClusterWide(t)
	i.ClusterWide = false
	previous, err := AllocateFor(context.Background(), i, p, "ip#1")
	require.NoError(t, err)
	ip, err := RedrawFor(context.Background(), i, p, "ip#1", previous)
	require.NoError(t, err)
	assert.NotEqual(t, previous.String(), ip.String())
}

// A redraw that fails holds the address it was to give up again, so the next
// one gives it up too rather than draw it.
func TestRedrawForFailingKeepsThePrevious(t *testing.T) {
	testhelper.Setup(t)
	p, _ := naming.ParsePath("ns1/svc/san1")
	i := newClusterWide(t)
	fakeClusterAddrs(t, nil, nil)
	previous, err := AllocateFor(context.Background(), i, p, "ip#1")
	require.NoError(t, err)
	lockNetwork = func(context.Context, string, string) (context.Context, func(), error) {
		return nil, nil, errors.New("cluster lock held")
	}
	_, err = RedrawFor(context.Background(), i, p, "ip#1", previous)
	require.Error(t, err)
	held, err := i.Allocated(ipam.Key(p, "ip#1"))
	require.NoError(t, err)
	assert.Equal(t, previous.String(), held.String())
}

// A draw whose lease ended before the address was reserved gives the address
// back and fails: another node may have drawn it meanwhile.
func TestAllocateForGivesBackADrawPastTheLease(t *testing.T) {
	testhelper.Setup(t)
	p, _ := naming.ParsePath("ns1/svc/san1")
	fakeClusterAddrs(t, nil, nil)
	lockNetwork = func(ctx context.Context, _, _ string) (context.Context, func(), error) {
		ended, cancel := context.WithCancel(ctx)
		cancel()
		return ended, func() {}, nil
	}
	i := newClusterWide(t)
	_, err := AllocateFor(context.Background(), i, p, "ip#1")
	assert.ErrorContains(t, err, "the lock lease ended")
	held, err := i.Allocated(ipam.Key(p, "ip#1"))
	require.NoError(t, err)
	assert.Nil(t, held, "the address was given back")
}
