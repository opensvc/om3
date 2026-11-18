package imon

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

var testVersion = time.Date(2026, 11, 17, 15, 33, 47, 0, time.UTC)

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

// A delete records the version of the config it removes when a scope node,
// a cluster node, may still hold it: not seen running it, away, or seen
// running it and holding the instance still. The object id is recorded.
func TestRecordTombstone(t *testing.T) {
	m, _, recorded := newTombstoneTestManager(t)
	m.instConfig.UpdatedAt = testVersion
	m.instConfig.ID = "o1"
	require.NoError(t, m.recordTombstone())
	require.Len(t, *recorded, 1)
	rec := (*recorded)[0]
	assert.Equal(t, m.state.OrchestrationID.String(), rec.OrchestrationID)
	assert.Equal(t, tombstone.KindDeleted, rec.Kind)
	assert.True(t, rec.Version.Equal(testVersion))
	assert.Equal(t, "o1", rec.ObjectID)
	assert.Equal(t, []string{"node2", "node3"}, m.mayHold(), "node4 is no cluster node")
}

// A peer seen running the delete holds nothing once its instance is gone,
// its node up; the last node to delete writes no record. A peer lost, or
// dropped as its data goes, proves nothing.
func TestRecordTombstoneSeenPeers(t *testing.T) {
	m, _, recorded := newTombstoneTestManager(t)
	m.instConfig.UpdatedAt = testVersion
	setNodeState(m, "node3", node.MonitorStateIdle)
	for _, n := range []string{"node2", "node3"} {
		m.onRemoteInstanceMonitorUpdated(&msgbus.InstanceMonitorUpdated{Path: m.path, Node: n, Value: instance.Monitor{
			OrchestrationID: m.state.OrchestrationID, GlobalExpect: instance.MonitorGlobalExpectDeleted}})
	}
	instance.ConfigData.Set(m.path, "node2", &instance.Config{})
	t.Cleanup(func() { instance.ConfigData.Unset(m.path, "node2") })
	assert.Equal(t, []string{"node2"}, m.mayHold(), "node2 holds the instance still")

	instance.ConfigData.Unset(m.path, "node2")
	require.NoError(t, m.recordTombstone())
	assert.Empty(t, *recorded, "every peer deleted already")

	setNodeState(m, "node3", node.MonitorStateMaintenance)
	assert.Equal(t, []string{"node3"}, m.mayHold(), "node3 lost: its data dropped proves nothing")
	setNodeState(m, "node3", node.MonitorStateIdle)
	node.DropNode("node3")
	assert.Equal(t, []string{"node3"}, m.mayHold(), "node3 dropped, the monitor not told yet")
}

// A delete whose tombstone can not be saved deletes nothing. The record
// lists the nodes that may hold the version deleted.
func TestCrmDeleteFailsWithoutItsTombstone(t *testing.T) {
	m, pub, _ := newTombstoneTestManager(t)
	var tried []tombstone.Record
	recordTombstoneFunc = func(rec tombstone.Record) error {
		tried = append(tried, rec)
		return errors.New("disk full")
	}
	m.instConfig.UpdatedAt = testVersion
	require.Error(t, m.crmDelete())
	require.Len(t, tried, 1)
	assert.Equal(t, []string{"node2", "node3"}, tried[0].Nodes)
	for _, msg := range pub.msgs {
		_, deleting := msg.(*msgbus.InstanceConfigDeleting)
		assert.False(t, deleting, "the delete did not start")
	}
}

// setKillers sets a record killing the local config of m, trusted or not.
func setKillers(m *Manager, trusted bool) tombstone.Record {
	m.instConfig.UpdatedAt = testVersion
	rec := tombstone.Record{OrchestrationID: uuid.New().String(), Path: m.path.String(), Kind: tombstone.KindDeleted,
		Version: testVersion, DeletedAt: testVersion}
	m.killers = []tombstone.Killer{{Node: "node2", Record: rec, Trusted: trusted}}
	return rec
}

// A record heard live killing the local config hands its delete to the
// instance, its stamp and id, when no orchestration runs and the node is
// up; a record read as the node returned does not, nor one of an
// orchestration the instance ran already, as one aborted.
func TestAdoptTombstone(t *testing.T) {
	m, _, _ := newTombstoneTestManager(t)
	m.state.GlobalExpect = instance.MonitorGlobalExpectNone
	m.state.OrchestrationID = uuid.Nil
	m.nodeMonitor[m.localhost] = node.Monitor{State: node.MonitorStateIdle}

	setKillers(m, false)
	assert.False(t, m.adoptTombstone(), "untrusted")

	rec := setKillers(m, true)
	m.logSetOrchestrationID(uuid.MustParse(rec.OrchestrationID))
	m.logSetOrchestrationID(uuid.New())
	assert.False(t, m.adoptTombstone(), "ran here already, another orchestration since, as an abort")
	m.carried = nil

	m.nodeMonitor[m.localhost] = node.Monitor{State: node.MonitorStateRejoin}
	assert.False(t, m.adoptTombstone(), "not before the rejoin")
	m.nodeMonitor[m.localhost] = node.Monitor{State: node.MonitorStateIdle}

	require.True(t, m.adoptTombstone())
	assert.Equal(t, instance.MonitorGlobalExpectDeleted, m.state.GlobalExpect)
	assert.Equal(t, rec.OrchestrationID, m.state.OrchestrationID.String())
	assert.True(t, m.state.GlobalExpectUpdatedAt.Equal(testVersion))
	assert.Empty(t, m.state.Issues)
}

// The issue shows when a record kills the local config and no delete of it
// is to come here: untrusted, or of an orchestration ran here, as one
// failed or aborted. It goes with a newer version, kept, or with the delete
// running.
func TestTombstoneIssue(t *testing.T) {
	m, _, _ := newTombstoneTestManager(t)
	m.state.GlobalExpect = instance.MonitorGlobalExpectNone
	rec := setKillers(m, false)
	issue, id := m.tombstoneIssue()
	assert.Contains(t, issue, "this version of the config deleted")
	assert.Equal(t, rec.OrchestrationID, id)

	rec = setKillers(m, true)
	issue, _ = m.tombstoneIssue()
	assert.Empty(t, issue, "its delete is to come here")
	m.logSetOrchestrationID(uuid.MustParse(rec.OrchestrationID))
	issue, _ = m.tombstoneIssue()
	assert.NotEmpty(t, issue, "failed or aborted here")

	m.state.GlobalExpect = instance.MonitorGlobalExpectDeleted
	issue, _ = m.tombstoneIssue()
	assert.Empty(t, issue, "its delete runs")

	m.state.GlobalExpect = instance.MonitorGlobalExpectNone
	m.killers = nil
	issue, _ = m.tombstoneIssue()
	assert.Empty(t, issue, "a newer version: no record kills it")
}

// The instance with the issue is flagged stopped once, once its status is
// known, and not if it is up; a flag not written is decided again.
func TestTombstoneStoppedFlag(t *testing.T) {
	m, pub, _ := newTombstoneTestManager(t)
	rawVar := rawconfig.Paths.Var
	t.Cleanup(func() { rawconfig.Paths.Var = rawVar })
	notADir := filepath.Join(t.TempDir(), "var")
	require.NoError(t, os.WriteFile(notADir, nil, 0o600))
	rawconfig.Paths.Var = notADir
	m.state.GlobalExpect = instance.MonitorGlobalExpectNone
	rec := setKillers(m, false)

	m.applyTombstoneIssue()
	require.Len(t, m.state.Issues, 1)
	assert.Empty(t, m.tombstoneStopped, "no status known yet")

	m.instStatus[m.localhost] = instance.Status{Avail: status.Down, UpdatedAt: time.Now()}
	m.applyTombstoneIssue()
	assert.Empty(t, m.tombstoneStopped, "the write failed")

	rawconfig.Paths.Var = t.TempDir()
	m.applyTombstoneIssue()
	assert.Equal(t, rec.OrchestrationID, m.tombstoneStopped)
	assert.True(t, m.isStopped())
	stopped := 0
	for _, msg := range pub.msgs {
		if _, ok := msg.(*msgbus.InstanceStoppedFileUpdated); ok {
			stopped++
		}
	}
	m.applyTombstoneIssue()
	n := 0
	for _, msg := range pub.msgs {
		if _, ok := msg.(*msgbus.InstanceStoppedFileUpdated); ok {
			n++
		}
	}
	assert.Equal(t, stopped, n, "flagged once per record")

	m2, _, _ := newTombstoneTestManager(t)
	m2.state.GlobalExpect = instance.MonitorGlobalExpectNone
	m2.instStatus[m2.localhost] = instance.Status{Avail: status.Up, UpdatedAt: time.Now()}
	setKillers(m2, false)
	m2.applyTombstoneIssue()
	assert.Len(t, m2.state.Issues, 1)
	assert.False(t, m2.isStopped(), "up: never flagged")
	ok, reason := m2.isRequestedStartable()
	assert.False(t, ok)
	assert.Contains(t, reason, "this version of the config deleted")
	assert.True(t, refusedByTombstone(instance.MonitorGlobalExpectProvisioned))
	assert.False(t, refusedByTombstone(instance.MonitorGlobalExpectPurged))
}

// A purge, a delete or an unprovision is refused naming the nodes away whose
// instance is up or provisioned, provisioned only for an unprovision; a
// node up, or a node with no status known, needs no force.
func TestAwayInstancesRefusal(t *testing.T) {
	m, _, _ := newTombstoneTestManager(t)
	// node3, in maintenance, published its instance up and not provisioned.
	m.instStatus["node3"] = instance.Status{Avail: status.Up, Provisioned: provisioned.False}
	m.instStatus["node2"] = instance.Status{Avail: status.Up, Provisioned: provisioned.True}
	reason := m.awayInstancesRefusal(instance.MonitorGlobalExpectPurged)
	assert.Contains(t, reason, "node3 away")
	assert.NotContains(t, reason, "node2", "node2 is up")
	assert.Contains(t, m.awayInstancesRefusal(instance.MonitorGlobalExpectDeleted), "node3")
	assert.Empty(t, m.awayInstancesRefusal(instance.MonitorGlobalExpectUnprovisioned), "up but not provisioned")

	m.instStatus["node3"] = instance.Status{Avail: status.Down, Provisioned: provisioned.True}
	assert.Contains(t, m.awayInstancesRefusal(instance.MonitorGlobalExpectUnprovisioned), "node3")
	delete(m.instStatus, "node3")
	assert.Empty(t, m.awayInstancesRefusal(instance.MonitorGlobalExpectPurged), "no status known")

	assert.True(t, forceAway(instance.MonitorGlobalExpectOptionsPurged{Force: true}))
	assert.True(t, forceAway(instance.MonitorGlobalExpectOptionsDeleted{Force: true}))
	assert.True(t, forceAway(instance.MonitorGlobalExpectOptionsUnprovisioned{Force: true}))
	assert.False(t, forceAway(nil))
}

// A node stopping deletes the statuses of its instances once it announced
// its shutdown: the last one is kept for the refusal, and forgotten as the
// node publishes again.
func TestAwayStatusKeptOnDeletion(t *testing.T) {
	m, _, _ := newTombstoneTestManager(t)
	m.nodeMonitor["node3"] = node.Monitor{State: node.MonitorStateShutdownProgress}
	m.instStatus["node3"] = instance.Status{Avail: status.Down, Provisioned: provisioned.True}
	m.onMyInstanceStatusDeleted(&msgbus.InstanceStatusDeleted{Path: m.path, Node: "node3"})
	assert.NotContains(t, m.instStatus, "node3")
	assert.Contains(t, m.awayInstancesRefusal(instance.MonitorGlobalExpectPurged), "node3")

	m.instStatus["node2"] = instance.Status{Provisioned: provisioned.True}
	m.onMyInstanceStatusDeleted(&msgbus.InstanceStatusDeleted{Path: m.path, Node: "node2"})
	assert.NotContains(t, m.awayStatus, "node2", "node2 is up: its instance is gone")

	m.onMyInstanceStatusUpdated("node3", &msgbus.InstanceStatusUpdated{Path: m.path, Node: "node3", Value: instance.Status{}})
	assert.NotContains(t, m.awayStatus, "node3")
}

// A peer dropped while up, as one crashed, loses its node data before its
// instance statuses, and its node monitor as this monitor heard it only at
// ForgetPeer: its last status is kept for the refusal, whichever of the
// status deletion and ForgetPeer comes first.
func TestAwayStatusKeptOnADrop(t *testing.T) {
	for _, forgetFirst := range []bool{false, true} {
		m, _, _ := newTombstoneTestManager(t)
		m.instStatus["node2"] = instance.Status{Avail: status.Up, Provisioned: provisioned.True}
		node.DropNode("node2")
		assert.Contains(t, m.awayInstancesRefusal(instance.MonitorGlobalExpectPurged), "node2", "dropped, not forgotten yet")
		if forgetFirst {
			m.onForgetPeer(&msgbus.ForgetPeer{Node: "node2"})
		}
		m.onMyInstanceStatusDeleted(&msgbus.InstanceStatusDeleted{Path: m.path, Node: "node2"})
		if !forgetFirst {
			m.onForgetPeer(&msgbus.ForgetPeer{Node: "node2"})
		}
		assert.Contains(t, m.awayInstancesRefusal(instance.MonitorGlobalExpectPurged), "node2", "forget first: %v", forgetFirst)
	}
}

// A peer back from away forgets the status it was away with.
func TestAwayStatusForgottenOnReturn(t *testing.T) {
	m, _, _ := newTombstoneTestManager(t)
	m.awayStatus = map[string]instance.Status{"node2": {Avail: status.Up}}
	m.onNodeMonitorUpdated(&msgbus.NodeMonitorUpdated{Node: "node2", Value: node.Monitor{State: node.MonitorStateMaintenance}})
	assert.Contains(t, m.awayStatus, "node2")
	m.onNodeMonitorUpdated(&msgbus.NodeMonitorUpdated{Node: "node2", Value: node.Monitor{State: node.MonitorStateIdle}})
	assert.NotContains(t, m.awayStatus, "node2")
}

// A node upgrading its packages, its daemon restarting, may hold the
// instance, and its instance up refuses a purge without --force.
func TestUpgradingNodeIsAway(t *testing.T) {
	m, _, _ := newTombstoneTestManager(t)
	setNodeState(m, "node2", node.MonitorStateUpgrade)
	assert.Contains(t, m.mayHold(), "node2")
	m.instStatus["node2"] = instance.Status{Avail: status.Up, Provisioned: provisioned.True}
	assert.Contains(t, m.awayInstancesRefusal(instance.MonitorGlobalExpectPurged), "node2")
}
