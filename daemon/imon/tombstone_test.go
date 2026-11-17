package imon

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

func newTombstoneTestManager(t *testing.T) (*Manager, *pubSpy, *[]tombstone.Record) {
	t.Helper()
	pub := &pubSpy{}
	m := newTestManager(pub)
	m.scopeNodes = []string{"node1", "node2", "node3", "node4"}
	m.nodeMonitor = make(map[string]node.Monitor)
	node.InitData()
	t.Cleanup(node.InitData)
	setNodeState(m, "node2", node.MonitorStateIdle)
	setNodeState(m, "node3", node.MonitorStateMaintenance)
	m.state.GlobalExpect = instance.MonitorGlobalExpectDeleted
	m.state.OrchestrationID = uuid.New()
	clusternode.Set([]string{"node1", "node2", "node3"})
	var recorded []tombstone.Record
	saved := recordTombstoneFunc
	recordTombstoneFunc = func(rec tombstone.Record) error {
		recorded = append(recorded, rec)
		return nil
	}
	t.Cleanup(func() { recordTombstoneFunc = saved })
	return m, pub, &recorded
}

// setNodeState sets the state of the node monitor of nodename, as the
// monitor heard it and in the node data.
func setNodeState(m *Manager, nodename string, state node.MonitorState) {
	m.nodeMonitor[nodename] = node.Monitor{State: state}
	node.MonitorData.Set(nodename, &node.Monitor{State: state})
}

// The scope nodes not seen running the delete are its holders: a node up
// live, a node in maintenance away, with its config checksum; a node out of
// the cluster none.
func TestRecordTombstoneHolders(t *testing.T) {
	m, _, recorded := newTombstoneTestManager(t)
	instance.ConfigData.Set(m.path, "node3", &instance.Config{})
	t.Cleanup(func() { instance.ConfigData.Unset(m.path, "node3") })
	require.NoError(t, m.recordTombstone())
	require.Len(t, *recorded, 1)
	rec := (*recorded)[0]
	assert.Equal(t, m.state.OrchestrationID.String(), rec.OrchestrationID)
	assert.Equal(t, tombstone.KindDeleted, rec.Kind)
	assert.Equal(t, map[string]tombstone.Holder{
		"node2": {State: tombstone.HolderPending, Mode: tombstone.HolderLive},
		"node3": {State: tombstone.HolderPending, Mode: tombstone.HolderAway},
	}, rec.Holders)
}

// A peer seen running the delete is a holder while it holds the instance,
// as killed before it deletes it would bring the object back; one up with no
// instance left is none, and the last node to delete writes no record. One
// lost, its data dropped with it, is an away holder.
func TestRecordTombstoneSeenPeers(t *testing.T) {
	m, _, recorded := newTombstoneTestManager(t)
	setNodeState(m, "node3", node.MonitorStateIdle)
	for _, n := range []string{"node2", "node3"} {
		m.onRemoteInstanceMonitorUpdated(&msgbus.InstanceMonitorUpdated{Path: m.path, Node: n, Value: instance.Monitor{
			OrchestrationID: m.state.OrchestrationID, GlobalExpect: instance.MonitorGlobalExpectDeleted}})
	}
	instance.ConfigData.Set(m.path, "node2", &instance.Config{})
	require.NoError(t, m.recordTombstone())
	require.Len(t, *recorded, 1)
	assert.Equal(t, []string{"node2"}, (*recorded)[0].PendingHolders(""), "node2 holds the instance still")

	instance.ConfigData.Unset(m.path, "node2")
	*recorded = nil
	require.NoError(t, m.recordTombstone())
	assert.Empty(t, *recorded, "every peer deleted already")

	setNodeState(m, "node3", node.MonitorStateMaintenance)
	require.NoError(t, m.recordTombstone())
	require.Len(t, *recorded, 1, "node3 lost: its data dropped proves nothing")
	assert.Equal(t, map[string]tombstone.Holder{"node3": {State: tombstone.HolderPending, Mode: tombstone.HolderAway}}, (*recorded)[0].Holders)
	delete(m.nodeMonitor, "node3")
	*recorded = nil
	require.NoError(t, m.recordTombstone())
	assert.Len(t, *recorded, 1, "node3 forgotten: the same")

	// Dropped: its node data goes first, the monitor not told yet.
	m.nodeMonitor["node3"] = node.Monitor{State: node.MonitorStateIdle}
	node.DropNode("node3")
	*recorded = nil
	require.NoError(t, m.recordTombstone())
	require.Len(t, *recorded, 1, "node3 dropped, the monitor not told yet: the same")
	assert.Equal(t, tombstone.HolderAway, (*recorded)[0].Holders["node3"].Mode)

	setNodeState(m, "node3", node.MonitorStateIdle)
	m.state.OrchestrationID = uuid.New()
	*recorded = nil
	require.NoError(t, m.recordTombstone())
	assert.Len(t, *recorded, 1, "seen carrying another orchestration")
}

// A delete whose tombstone can not be saved deletes nothing.
func TestCrmDeleteFailsWithoutItsTombstone(t *testing.T) {
	m, pub, _ := newTombstoneTestManager(t)
	recordTombstoneFunc = func(tombstone.Record) error { return errors.New("disk full") }
	require.Error(t, m.crmDelete())
	for _, msg := range pub.msgs {
		_, deleting := msg.(*msgbus.InstanceConfigDeleting)
		assert.False(t, deleting, "the delete did not start")
	}
}
