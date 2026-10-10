//go:build linux

package networkroutedbridge

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The tunnels between the same two addresses are one, whatever the network,
// and the tunnels between other addresses are others, with names a link may
// take.
func TestTunName(t *testing.T) {
	a, b, c := net.ParseIP("fd01::11"), net.ParseIP("fd01::12"), net.ParseIP("fdfe::1")
	assert.Equal(t, tunName(a, b), tunName(a, b))
	assert.NotEqual(t, tunName(a, b), tunName(c, b))
	assert.NotEqual(t, tunName(a, b), tunName(b, a))
	assert.LessOrEqual(t, len(tunName(a, b)), 15)
	assert.Equal(t, "tun1029012", tunName(net.ParseIP("10.29.0.11"), net.ParseIP("10.29.0.12")))
}
