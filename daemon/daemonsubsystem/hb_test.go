package daemonsubsystem

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHeartbeatUnstructuredHoldsNoEscapeSequence pins the layer this type
// sits in. It is published by the daemon, read by the tui and by the api
// clients, and it used to compose the green "O" and the red "X" a listing
// draws, which are escape sequences. The listing draws them now, from the
// values here.
func TestHeartbeatUnstructuredHoldsNoEscapeSequence(t *testing.T) {
	for _, state := range []string{"running", "stopped", "failed", "warning", "", "unknown"} {
		for _, isBeating := range []bool{true, false} {
			for _, isSingleNode := range []bool{true, false} {
				entry := HeartbeatStreamPeerStatusTableEntry{
					Node: "n1",
					Peer: "n2",
					Type: "unicast",
					HeartbeatStreamPeerStatus: HeartbeatStreamPeerStatus{
						IsBeating: isBeating,
					},
					IsSingleNode: isSingleNode,
				}
				entry.Status.State = state
				for key, value := range entry.Unstructured() {
					s, ok := value.(string)
					if !ok {
						continue
					}
					assert.NotContainsf(t, s, "\x1b", "%s holds an escape sequence: %q", key, s)
				}
			}
		}
	}
}

// TestHeartbeatUnstructuredKeepsTheValuesTheListingDrawsFrom pins the keys
// the icons are drawn from, so removing one is a test failure rather than
// an empty column.
func TestHeartbeatUnstructuredKeepsTheValuesTheListingDrawsFrom(t *testing.T) {
	entry := HeartbeatStreamPeerStatusTableEntry{}
	entry.Status.State = "running"
	entry.IsBeating = true
	m := entry.Unstructured()
	assert.Equal(t, "running", m["state"])
	assert.Equal(t, true, m["is_beating"])
	assert.Equal(t, "beating", m["beating"])

	entry.IsBeating = false
	assert.Equal(t, "stale", entry.Unstructured()["beating"])
}

// TestHeartbeatUnstructuredNamesTheUnknownState pins that an empty state
// reads as a word rather than as nothing.
func TestHeartbeatUnstructuredNamesTheUnknownState(t *testing.T) {
	entry := HeartbeatStreamPeerStatusTableEntry{}
	m := entry.Unstructured()
	assert.Equal(t, "unknown", m["state"])
	assert.Equal(t, "N/A", m["peer"])
	assert.Equal(t, "N/A", m["desc"])
}

// A peer powered off without closing its connections is still beating on
// a tx stream: only the rx streams tell it is awake.
func TestHeartbeatIsReceiving(t *testing.T) {
	hb := Heartbeat{Streams: []HeartbeatStream{
		{Peers: map[string]HeartbeatStreamPeerStatus{"n2": {IsBeating: false}, "n3": {IsBeating: true}}},
		{Peers: map[string]HeartbeatStreamPeerStatus{"n2": {IsBeating: true}, "n3": {IsBeating: true}}},
	}}
	hb.Streams[0].ID = "hb#1.rx"
	hb.Streams[1].ID = "hb#1.tx"
	assert.False(t, hb.IsReceiving("n2"), "beating on tx only")
	assert.True(t, hb.IsReceiving("n3"), "beating on rx")
	assert.False(t, hb.IsReceiving("n4"), "unknown")
}
