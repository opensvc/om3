package hbucast

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

// TestPeerAddr pins the address the tx dials: an IPv6 one joined to its port
// as "addr:port" could not be dialed.
func TestPeerAddr(t *testing.T) {
	assert.Equal(t, "10.0.0.2:10000", peerAddr("n2", "10.0.0.2", 10000))
	assert.Equal(t, "[fd01::12]:10000", peerAddr("n2", "fd01::12", 10000))
	assert.Equal(t, "n2:10000", peerAddr("n2", "", 10000), "a node with no addr is dialed by its name")
	assert.Equal(t, "fd01::12", peerAddr("n2", "fd01::12", 0))
}

func TestListenAddrAndDesc(t *testing.T) {
	for name, tc := range map[string]struct {
		addr   string
		intf   string
		peer   string
		listen string
		desc   string
	}{
		"IPv4": {
			addr: "10.0.0.1", peer: "10.0.0.2:10000",
			listen: "10.0.0.1:10000", desc: "10.0.0.1:10000 ← 10.0.0.2",
		},
		"IPv6": {
			addr: "fd01::11", peer: "[fd01::12]:10000",
			listen: "[fd01::11]:10000", desc: "[fd01::11]:10000 ← fd01::12",
		},
		"IPv6 with intf": {
			addr: "fd01::11", intf: "eth0", peer: "[fd01::12]:10000",
			listen: "[fd01::11]:10000", desc: "[fd01::11]:10000@eth0 ← fd01::12",
		},
		"no addr": {
			peer:   "[fd01::12]:10000",
			listen: ":10000", desc: ":10000 ← fd01::12",
		},
		"no addr with intf, peer by name": {
			intf: "eth0", peer: "n2:10000",
			listen: ":10000", desc: ":10000@eth0 ← n2",
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := &rx{addr: tc.addr, port: "10000", intf: tc.intf}
			assert.Equal(t, tc.listen, r.listenAddr())
			assert.Equal(t, tc.desc, r.streamPeerDesc(tc.peer))
		})
	}
}

func TestCanonicalIP(t *testing.T) {
	assert.Equal(t, "fd01::11", canonicalIP("FD01:0:0:0::11"))
	assert.Equal(t, "fd01::11", canonicalIP("fd01:0000::0011"))
	assert.Equal(t, "10.0.0.1", canonicalIP("::ffff:10.0.0.1"), "an IPv4-mapped address is its IPv4 one")
	assert.Equal(t, "10.0.0.1", canonicalIP("10.0.0.1"))
	assert.Equal(t, "n2", canonicalIP("n2"), "a name is kept as is")
}

// TestResolvePeerIPsIPv6 pins the allow-list of the rx on IPv6 peers: the
// address cut at its first colon was "fd01", which resolves to nothing, and
// every peer connection was refused.
func TestResolvePeerIPsIPv6(t *testing.T) {
	ctx := context.Background()
	r := &rx{
		nodes: map[string]string{
			"n2": "[fd01::12]:10000",
			"n3": "[FD01:0:0:0::13]:10000",
			"n4": "fd01::14",
		},
		log: plog.NewDefaultLogger(),
	}
	got := r.resolvePeerIPs(ctx, nil)
	require.Equal(t, map[string][]string{
		"n2": {"fd01::12"},
		"n3": {"fd01::13"},
		"n4": {"fd01::14"},
	}, got, "a literal is resolved to itself, written as a connection address is")
	set, _ := peerIPSet(got)
	// what the accept loop checks: the address of the connection, split
	// from its port and canonicalised
	assert.Contains(t, set, canonicalIP("fd01::13"))
	assert.Contains(t, set, canonicalIP("FD01::13"))
}
