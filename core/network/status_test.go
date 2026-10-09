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

// A network whose range does not parse, as one declared with a type that
// reads no range, used to leave its counts nil, and the API handler
// dereferencing them panicked for every network listing.
func TestGetStatusOfANetworkWithoutRange(t *testing.T) {
	nw := &T{}
	nw.SetName("broken")
	nw.SetDriver("lo")
	nw.SetNetwork("")
	s := GetStatus(nw, clusterip.L{})
	if assert.NotNil(t, s.Size) && assert.NotNil(t, s.Used) && assert.NotNil(t, s.Free) {
		assert.Equal(t, "0", s.Size.String())
		assert.Equal(t, "0", s.Free.String())
	}
	assert.Len(t, s.Errors, 1)
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
