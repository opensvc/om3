package network

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusterip"
	"github.com/opensvc/om3/v3/core/ipam"
)

// fakePooled is a network handing out the pools of its segment only, which
// must be in the segment, as the ones of a lan network.
type fakePooled struct {
	*T
	pools []string
}

func (t *fakePooled) Pools() ([]ipam.Pool, error) {
	segment, err := t.IPNet()
	if err != nil {
		return nil, err
	}
	pools, err := ipam.ParsePools(t.pools)
	if err != nil {
		return nil, err
	}
	for _, p := range pools {
		if !p.In(segment) {
			return nil, fmt.Errorf("%s is not in the segment %s", p, segment)
		}
	}
	return pools, nil
}

func newFakePooled(name, network string, pools ...string) *fakePooled {
	t := &T{}
	t.SetName(name)
	t.SetNetwork(network)
	return &fakePooled{T: t, pools: pools}
}

func newFakeSubnet(name, network string) *T {
	t := &T{}
	t.SetName(name)
	t.SetNetwork(network)
	return t
}

// Two networks of the same segment overlap only when their pools do, and a
// subnet overlaps the pools of a segment it holds addresses of.
func TestCheckOverlapByPools(t *testing.T) {
	a := newFakePooled("a", "192.168.10.0/24", "192.168.10.100-192.168.10.149")
	b := newFakePooled("b", "192.168.10.0/24", "192.168.10.150-192.168.10.199")
	c := newFakePooled("c", "192.168.10.0/24", "192.168.10.128/26")
	assert.Empty(t, Check([]Networker{a, b}))
	errs := Check([]Networker{a, c})
	assert.NoError(t, errs["a"], "the network declared first keeps working")
	assert.ErrorContains(t, errs["c"], "overlaps")

	inside := newFakeSubnet("inside", "192.168.10.64/27")
	assert.Empty(t, Check([]Networker{a, inside}), "the subnet holds none of the pool")
	over := newFakeSubnet("over", "192.168.10.96/27")
	assert.ErrorContains(t, Check([]Networker{a, over})["over"], "overlaps")
}

// A network whose ranges leave its segment is refused for that, not taken
// for its whole segment, which would say it overlaps the networks of the
// segment, and would refuse them too.
func TestCheckSaysTheRangesLeavingTheSegment(t *testing.T) {
	lan := newFakePooled("lan", "10.32.0.0/24", "10.32.0.200-10.32.0.239")
	bad := newFakePooled("bad", "10.32.0.0/24", "10.33.0.200-10.33.0.210")
	errs := Check([]Networker{lan, bad})
	assert.NoError(t, errs["lan"])
	assert.ErrorContains(t, errs["bad"], "is not in the segment")
}

// The size of a network handing out pools of its segment is the one of its
// pools, and the addresses used are the ones of the pools.
func TestStatusOfPools(t *testing.T) {
	nw := newFakePooled("a", "192.168.10.0/24", "192.168.10.100-192.168.10.149", "192.168.10.151-192.168.10.199")
	ips := clusterip.L{
		{IP: []byte{192, 168, 10, 11}, Node: "n1"},
		{IP: []byte{192, 168, 10, 120}, Node: "n1"},
	}
	data := GetStatus(nw, ips)
	require.Empty(t, data.Errors)
	assert.Equal(t, int64(99), data.Size.Int64())
	assert.Equal(t, int64(1), data.Used.Int64())
	assert.Equal(t, int64(98), data.Free.Int64())
}
