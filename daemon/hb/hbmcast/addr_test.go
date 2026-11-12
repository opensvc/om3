package hbmcast

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/hbcfg"
)

// TestGroupAddr pins the resolution of the group a multicast heartbeat
// joins: an IPv6 group joined to its port as "addr:port" did not resolve,
// "too many colons", and the heartbeat never started.
func TestGroupAddr(t *testing.T) {
	t.Run("an IPv4 group", func(t *testing.T) {
		udpAddr, err := groupAddr("224.3.29.71", 10000, "")
		require.NoError(t, err)
		assert.True(t, udpAddr.IP.IsMulticast())
		assert.NotNil(t, udpAddr.IP.To4())
		assert.Equal(t, "224.3.29.71:10000", udpAddr.String())
		assert.Equal(t, "udp4", udpNetwork(udpAddr))
	})
	t.Run("an IPv6 group with intf", func(t *testing.T) {
		udpAddr, err := groupAddr("ff02::1:4141", 10006, "eth0")
		require.NoError(t, err)
		assert.True(t, udpAddr.IP.IsMulticast())
		assert.Nil(t, udpAddr.IP.To4())
		assert.Equal(t, "[ff02::1:4141]:10006", udpAddr.String(), "the stream desc is the address bracketed")
		assert.Equal(t, "udp6", udpNetwork(udpAddr))
	})
	t.Run("an IPv6 group without intf", func(t *testing.T) {
		_, err := groupAddr("ff02::1:4141", 10006, "")
		require.Error(t, err)
		assert.True(t, errors.Is(err, hbcfg.ErrIntfRequired))
		assert.Equal(t, "the IPv6 group ff02::1:4141 needs intf", err.Error())
	})
	t.Run("an IPv6 group naming a zone", func(t *testing.T) {
		_, err := groupAddr("ff02::1:4141%eth0", 10006, "eth0")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name the interface with intf")
	})
	t.Run("a unicast address", func(t *testing.T) {
		_, err := groupAddr("10.0.0.1", 10000, "")
		require.Error(t, err)
		assert.Equal(t, "10.0.0.1 is not a multicast group", err.Error())
	})
}
