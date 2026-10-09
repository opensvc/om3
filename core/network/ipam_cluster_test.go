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
