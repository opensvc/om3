package commoncmd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

// wakeClusterData is the status of the cluster n1 n2 n3 n4 seen by the api
// node n1: n3 beats, n2 and n4 do not.
func wakeClusterData() clusterdump.Data {
	var data clusterdump.Data
	data.Daemon.Nodename = "n1"
	data.Cluster.Config.Nodes = []string{"n1", "n2", "n3", "n4"}
	var n node.Node
	n.Daemon.Heartbeat.Streams = []daemonsubsystem.HeartbeatStream{
		{Peers: map[string]daemonsubsystem.HeartbeatStreamPeerStatus{"n2": {}, "n3": {IsBeating: true}, "n4": {}}},
		{Peers: map[string]daemonsubsystem.HeartbeatStreamPeerStatus{"n2": {IsBeating: true}, "n3": {IsBeating: true}, "n4": {IsBeating: true}}},
	}
	n.Daemon.Heartbeat.Streams[0].ID = "hb#1.rx"
	n.Daemon.Heartbeat.Streams[1].ID = "hb#1.tx"
	data.Cluster.Node = map[string]node.Node{"n1": n}
	return data
}

func TestExpandClusterNodes(t *testing.T) {
	nodes := []string{"dev4n1", "dev4n2", "dev5n1"}
	cases := map[string][]string{
		"dev4n2":        {"dev4n2"},
		"DEV4N2":        {"dev4n2"},
		"dev4n2,dev4n1": {"dev4n2", "dev4n1"},
		"dev4n2 dev4n2": {"dev4n2"},
		"dev4*":         {"dev4n1", "dev4n2"},
		"dev4n2,dev*":   {"dev4n2", "dev4n1", "dev5n1"},
		"dev?n1":        {"dev4n1", "dev5n1"},
	}
	for selector, want := range cases {
		got, err := expandClusterNodes(selector, nodes)
		require.NoError(t, err, selector)
		assert.Equal(t, want, got, selector)
	}
	for _, selector := range []string{"", "dev9n1", "dev4n2,dev9n1", "x*"} {
		_, err := expandClusterNodes(selector, nodes)
		assert.Error(t, err, selector)
	}
}

func TestWakeRelays(t *testing.T) {
	data := wakeClusterData()
	assert.Equal(t, []string{"n1", "n3"}, wakeRelays(data, "n2", nil), "every peer up but the target")
	assert.Equal(t, []string{"n3"}, wakeRelays(data, "n2", []string{"n3", "n4"}), "the configured relays up")
	assert.Equal(t, []string{"n1"}, wakeRelays(data, "n2", []string{"n1", "n1", "n2"}), "no duplicate, never the target")
	assert.Empty(t, wakeRelays(data, "n2", []string{"n4"}), "no configured relay up")
}

func TestWaitNodesAlive(t *testing.T) {
	t.Run("returns when every node is alive", func(t *testing.T) {
		r := &fakeEventReader{msgs: []any{
			&msgbus.NodeAlive{Node: "n9"},
			&msgbus.NodeAlive{Node: "n2"},
			&msgbus.NodeAlive{Node: "n2"},
			&msgbus.NodeAlive{Node: "n4"},
		}}
		var err error
		out := captureStdout(t, func() {
			err = waitNodesAlive(context.Background(), r, []string{"n2", "n4"}, time.Now())
		})
		require.NoError(t, err)
		assert.Contains(t, out, "n2: awake after")
		assert.Contains(t, out, "n4: awake after")
		assert.NotContains(t, out, "n9")
	})
	t.Run("names the nodes not alive when the stream ends", func(t *testing.T) {
		r := &fakeEventReader{msgs: []any{&msgbus.NodeAlive{Node: "n2"}}}
		var err error
		captureStdout(t, func() {
			err = waitNodesAlive(context.Background(), r, []string{"n2", "n4"}, time.Now())
		})
		assert.ErrorContains(t, err, "n4: heartbeat not beating yet")
	})
	t.Run("names the nodes not alive at the timeout", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := waitNodesAlive(ctx, &fakeEventReader{}, []string{"n2"}, time.Now())
		assert.ErrorContains(t, err, "n2: heartbeat not beating yet")
	})
}

// A relays value written with commas, as "n1," or "n1,n3", names nodes.
func TestSplitNodeSelector(t *testing.T) {
	assert.Equal(t, []string{"n1"}, splitNodeSelector("n1,"))
	assert.Equal(t, []string{"n1", "n3"}, splitNodeSelector("n1,n3"))
	assert.Equal(t, []string{"n1", "n3"}, splitNodeSelector(" n1  n3 "))
	assert.Empty(t, splitNodeSelector(""))
}

// A peer powered off without closing its connections is still beating on
// the tx stream of the api node: only the rx streams tell it is awake.
func TestIsReceiving(t *testing.T) {
	var data clusterdump.Data
	data.Daemon.Nodename = "n1"
	var n node.Node
	n.Daemon.Heartbeat.Streams = []daemonsubsystem.HeartbeatStream{
		{Peers: map[string]daemonsubsystem.HeartbeatStreamPeerStatus{"n2": {IsBeating: false}, "n3": {IsBeating: true}}},
		{Peers: map[string]daemonsubsystem.HeartbeatStreamPeerStatus{"n2": {IsBeating: true}, "n3": {IsBeating: true}}},
	}
	n.Daemon.Heartbeat.Streams[0].ID = "hb#1.rx"
	n.Daemon.Heartbeat.Streams[1].ID = "hb#1.tx"
	data.Cluster.Node = map[string]node.Node{"n1": n}

	assert.True(t, isReceiving(data, "n1"), "the api node")
	assert.False(t, isReceiving(data, "n2"), "beating on tx only")
	assert.True(t, isReceiving(data, "n3"), "beating on rx")
	assert.False(t, isReceiving(data, "n4"), "unknown")
}
