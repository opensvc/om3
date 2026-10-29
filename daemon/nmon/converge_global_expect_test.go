package nmon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/node"
)

// A daemon restarted after a cluster thaw ended reports the zero time of the
// global expect, and its peers, which ended the thaw at its time, waited for
// it to have that time before they handled their local expect, as a drain.
// The restarted daemon adopts the time of the global expect the peers ended,
// and the peers converge.
func TestAlignEndedGlobalExpect(t *testing.T) {
	thawedAt := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	ended := node.Monitor{GlobalExpect: node.MonitorGlobalExpectNone, GlobalExpectUpdatedAt: thawedAt}

	restarted := newTestManager(&recordingPublisher{})
	restarted.state.GlobalExpect = node.MonitorGlobalExpectNone
	restarted.nodeMonitor = map[string]node.Monitor{
		"node1": restarted.state,
		"node2": ended,
		"node3": ended,
	}
	restarted.convergeGlobalExpectFromRemote()
	assert.Equal(t, thawedAt, restarted.state.GlobalExpectUpdatedAt, "the time of the thaw the peers ended")
	assert.Equal(t, node.MonitorGlobalExpectNone, restarted.state.GlobalExpect, "no global expect adopted")

	peer := newTestManager(&recordingPublisher{})
	peer.localhost = "node3"
	peer.state = ended
	peer.nodeMonitor = map[string]node.Monitor{
		"node1": {GlobalExpect: node.MonitorGlobalExpectNone},
		"node2": ended,
		"node3": ended,
	}
	assert.False(t, peer.isConvergedGlobalExpect(), "the restarted node not aligned yet")
	peer.nodeMonitor["node1"] = restarted.state
	assert.True(t, peer.isConvergedGlobalExpect(), "the restarted node aligned")

	// A global expect pending here is the orchestration's to end: the time
	// of one a peer ended is not adopted.
	frozenAt := thawedAt.Add(-time.Hour)
	pending := newTestManager(&recordingPublisher{})
	pending.state = node.Monitor{GlobalExpect: node.MonitorGlobalExpectFrozen, GlobalExpectUpdatedAt: frozenAt}
	pending.nodeMonitor = map[string]node.Monitor{"node1": pending.state, "node2": ended}
	pending.convergeGlobalExpectFromRemote()
	assert.Equal(t, frozenAt, pending.state.GlobalExpectUpdatedAt)
}
