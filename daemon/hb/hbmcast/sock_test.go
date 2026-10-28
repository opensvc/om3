package hbmcast

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A datagram sent by the sending socket is read by a receiver of the group,
// on the interface the system picks when none is configured. The test is
// skipped on a host with no route for the group.
func TestMulticastRoundTrip(t *testing.T) {
	gaddr := &net.UDPAddr{IP: net.ParseIP("239.255.83.71"), Port: freeUDPPort(t)}
	l, err := net.ListenMulticastUDP("udp", nil, gaddr)
	if err != nil {
		t.Skipf("no multicast here: %s", err)
	}
	defer l.Close()
	c, err := dialMulticast(nil, gaddr)
	require.NoError(t, err)
	defer c.Close()
	if _, err := c.WriteToUDP([]byte("beat"), gaddr); err != nil {
		t.Skipf("no multicast route here: %s", err)
	}
	require.NoError(t, l.SetReadDeadline(time.Now().Add(2*time.Second)))
	b := make([]byte, 16)
	n, _, err := l.ReadFromUDP(b)
	require.NoError(t, err)
	assert.Equal(t, "beat", string(b[:n]))
}

func TestHasAddrOf(t *testing.T) {
	lo, err := net.InterfaceByName("lo")
	if err != nil {
		t.Skipf("no lo interface: %s", err)
	}
	assert.True(t, hasAddrOf(lo, &net.UDPAddr{IP: net.ParseIP("224.3.29.71")}), "lo has 127.0.0.1")
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero})
	require.NoError(t, err)
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).Port
}
