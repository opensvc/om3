package network

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/clusterip"
)

func TestGetStatusOfAValidNetwork(t *testing.T) {
	nw := &T{}
	nw.SetName("default")
	nw.SetDriver("bridge")
	nw.SetNetwork("10.22.0.0/16")
	ips := clusterip.L{{IP: net.ParseIP("10.22.0.5")}, {IP: net.ParseIP("10.23.0.5")}}
	s := GetStatus(nw, ips)
	assert.Equal(t, "65536", s.Size.String())
	assert.Equal(t, "1", s.Used.String())
	assert.Equal(t, "65535", s.Free.String())
	assert.Empty(t, s.Errors)
}

// A network whose range does not parse used to leave its counts nil, and the
// API handler dereferencing them panicked for every network listing.
func TestGetStatusOfANetworkWithAMalformedRange(t *testing.T) {
	nw := &T{}
	nw.SetName("broken")
	nw.SetDriver("bridge")
	nw.SetNetwork("10.99.0.0/99")
	s := GetStatus(nw, clusterip.L{})
	if assert.NotNil(t, s.Size) && assert.NotNil(t, s.Used) && assert.NotNil(t, s.Free) {
		assert.Equal(t, "0", s.Size.String())
		assert.Equal(t, "0", s.Free.String())
	}
	assert.Len(t, s.Errors, 1)
}

// A network whose driver allows no range, as the loopback one, is valid: it
// has zero counts and no error.
func TestGetStatusOfANetworkAllowedNoRange(t *testing.T) {
	nw := &T{}
	nw.SetName("lo2")
	nw.SetDriver("lo")
	nw.SetAllowEmptyNetwork(true)
	nw.SetNetwork("")
	s := GetStatus(nw, clusterip.L{})
	if assert.NotNil(t, s.Size) && assert.NotNil(t, s.Used) && assert.NotNil(t, s.Free) {
		assert.Equal(t, "0", s.Size.String())
		assert.Equal(t, "0", s.Used.String())
	}
	assert.Empty(t, s.Errors)
	assert.True(t, IsValid(nw), "GetStatus and IsValid agree")
}

func TestGetStatusWithoutAddresses(t *testing.T) {
	nw := &T{}
	nw.SetName("default")
	nw.SetDriver("bridge")
	nw.SetNetwork("10.22.0.0/16")
	s := GetStatus(nw, nil)
	assert.Equal(t, "65536", s.Size.String())
	assert.Equal(t, "0", s.Used.String())
}

type clusterWideNetwork struct {
	Networker
	wide bool
}

func (t clusterWideNetwork) IsClusterWide() bool { return t.wide }

// Every node of a cluster-wide network reports the address a failover object
// holds, which is one address. The same address on two nodes of a node local
// network is two.
func TestUsedCount(t *testing.T) {
	ips := clusterip.L{
		{IP: net.ParseIP("fd01::5:1"), Node: "n1"},
		{IP: net.ParseIP("fd01::5:1"), Node: "n2"},
		{IP: net.ParseIP("fd01::5:2"), Node: "n1"},
	}
	if got := usedCount(clusterWideNetwork{wide: true}, ips); got != 2 {
		t.Errorf("cluster-wide: got %d, want 2", got)
	}
	if got := usedCount(clusterWideNetwork{wide: false}, ips); got != 3 {
		t.Errorf("node local: got %d, want 3", got)
	}
}
