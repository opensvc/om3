package nmon

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type pubRecorder struct {
	msgs []pubsub.Messager
}

func (t *pubRecorder) Pub(m pubsub.Messager, _ ...pubsub.Label) {
	t.msgs = append(t.msgs, m)
}

func (t *pubRecorder) ends() []*msgbus.NodeOrchestrationEnd {
	var l []*msgbus.NodeOrchestrationEnd
	for _, m := range t.msgs {
		if e, ok := m.(*msgbus.NodeOrchestrationEnd); ok {
			l = append(l, e)
		}
	}
	return l
}

// newEndingTestManager returns the monitor of node1 whose part of the
// cluster orchestration of id, for globalExpect, is done, its peers carrying
// it as given.
func newEndingTestManager(pub *pubRecorder, globalExpect node.MonitorGlobalExpect, id uuid.UUID, peers map[string]node.Monitor) *Manager {
	m := &Manager{
		localhost:               "node1",
		log:                     plog.NewDefaultLogger(),
		publisher:               pub,
		nodeMonitor:             peers,
		orchestrationClearTimer: time.NewTimer(time.Hour),
		state:                   node.Monitor{State: node.MonitorStateIdle, GlobalExpect: globalExpect, OrchestrationID: id},
	}
	m.orchestrationClearTimer.Stop()
	m.setNextPendingOrchestration()
	return m
}

// The end of a cluster freeze done here, which ends the --wait of the
// requester, waits for the peers still freezing: it said the cluster frozen
// with its peers mid-way, and a thaw asked then reached them there.
func TestTheEndOfAClusterFreezeWaitsForThePeers(t *testing.T) {
	pub := &pubRecorder{}
	id := uuid.New()
	m := newEndingTestManager(pub, node.MonitorGlobalExpectFrozen, id, map[string]node.Monitor{
		"node2": {State: node.MonitorStateFreezeProgress, OrchestrationID: id},
		"node3": {State: node.MonitorStateMaintenance, OrchestrationID: id},
	})
	m.endOrchestration()
	assert.Equal(t, uuid.Nil, m.state.OrchestrationID, "cleared here at once")
	assert.Empty(t, pub.ends(), "node2 is freezing")

	m.nodeMonitor["node2"] = node.Monitor{State: node.MonitorStateIdle}
	m.onOrchestrationEndingCheck()
	require.Len(t, pub.ends(), 1, "node2 done, node3 in maintenance not waited for")
	assert.Equal(t, id.String(), pub.ends()[0].ID)
	assert.False(t, pub.ends()[0].Failed)
}

// A peer failing its part ends the wait failed, naming it.
func TestTheEndOfAClusterFreezeSaysAPeerFailed(t *testing.T) {
	pub := &pubRecorder{}
	id := uuid.New()
	m := newEndingTestManager(pub, node.MonitorGlobalExpectFrozen, id, map[string]node.Monitor{
		"node2": {State: node.MonitorStateFreezeFailure, OrchestrationID: id},
	})
	m.endOrchestration()
	require.Len(t, pub.ends(), 1)
	assert.True(t, pub.ends()[0].Failed)
	assert.Equal(t, "freeze failed on node2", pub.ends()[0].Error)
}

// A peer not done in time ends the wait failed, naming it; a peer forgotten
// is not waited for.
func TestTheEndOfAClusterFreezeIsBounded(t *testing.T) {
	pub := &pubRecorder{}
	id := uuid.New()
	m := newEndingTestManager(pub, node.MonitorGlobalExpectFrozen, id, map[string]node.Monitor{
		"node2": {State: node.MonitorStateFreezeProgress, OrchestrationID: id},
	})
	m.endOrchestration()
	m.orchestrationEndingSince = time.Now().Add(-orchestrationEndingTimeout)
	m.onOrchestrationEndingCheck()
	require.Len(t, pub.ends(), 1)
	assert.True(t, pub.ends()[0].Failed)
	assert.Contains(t, pub.ends()[0].Error, "node2")

	pub = &pubRecorder{}
	m = newEndingTestManager(pub, node.MonitorGlobalExpectUnfrozen, id, map[string]node.Monitor{
		"node2": {State: node.MonitorStateUnfreezeProgress, OrchestrationID: id},
	})
	m.endOrchestration()
	delete(m.nodeMonitor, "node2")
	m.onOrchestrationEndingCheck()
	require.Len(t, pub.ends(), 1, "node2 gone")
	assert.False(t, pub.ends()[0].Failed)
}

// A new orchestration displacing the one whose end is held back ends it
// aborted; a drain, of this node alone, ends at once.
func TestTheEndHeldBackIsAbortedByANewOrchestration(t *testing.T) {
	pub := &pubRecorder{}
	id := uuid.New()
	m := newEndingTestManager(pub, node.MonitorGlobalExpectFrozen, id, map[string]node.Monitor{
		"node2": {State: node.MonitorStateFreezeProgress, OrchestrationID: id},
	})
	m.endOrchestration()
	m.state.OrchestrationID = uuid.New()
	m.setNextPendingOrchestration()
	require.Len(t, pub.ends(), 1)
	assert.True(t, pub.ends()[0].Aborted)

	pub = &pubRecorder{}
	m = newEndingTestManager(pub, node.MonitorGlobalExpectNone, id, map[string]node.Monitor{
		"node2": {State: node.MonitorStateIdle, OrchestrationID: id},
	})
	m.endOrchestration()
	assert.Len(t, pub.ends(), 1, "not a cluster orchestration: at once")
}
