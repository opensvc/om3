package hbucast

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

func TestPeerIPSet(t *testing.T) {
	set, list := peerIPSet(map[string][]string{
		"node2": {"10.0.0.2", "10.0.1.2"},
		// node3 has a floating address it shares with node2
		"node3": {"10.0.1.2", "10.0.0.3"},
	})
	require.Equal(t, []string{"10.0.0.2", "10.0.0.3", "10.0.1.2"}, list,
		"the list is sorted and holds each address once")
	for _, addr := range list {
		require.Contains(t, set, addr)
	}
	require.Len(t, set, len(list))

	set, list = peerIPSet(nil)
	require.Empty(t, set)
	require.Empty(t, list)
}

func TestResolvePeerIPs(t *testing.T) {
	ctx := context.Background()
	// a name reserved by RFC 6761 for the purpose, it must not resolve
	unresolvable := "node2.invalid"
	if _, err := net.DefaultResolver.LookupHost(ctx, unresolvable); err == nil {
		t.Skipf("%s resolves on this node, the lookup failure path can't be tested", unresolvable)
	}

	newRx := func(nodes map[string]string) *rx {
		return &rx{nodes: nodes, log: plog.NewDefaultLogger()}
	}

	t.Run("an address is resolved to itself", func(t *testing.T) {
		r := newRx(map[string]string{"node2": "10.0.0.2:10000"})
		require.Equal(t, map[string][]string{"node2": {"10.0.0.2"}}, r.resolvePeerIPs(ctx, nil))
	})

	t.Run("a node that doesn't resolve has no address", func(t *testing.T) {
		r := newRx(map[string]string{"node2": unresolvable + ":10000"})
		require.Empty(t, r.resolvePeerIPs(ctx, nil))
	})

	t.Run("a node that stops resolving keeps its known addresses", func(t *testing.T) {
		r := newRx(map[string]string{"node2": unresolvable + ":10000"})
		previous := map[string][]string{"node2": {"10.0.0.2"}}
		require.Equal(t, previous, r.resolvePeerIPs(ctx, previous),
			"a transient resolver failure must not empty the allow list")
	})
}
