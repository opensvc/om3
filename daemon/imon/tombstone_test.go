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

func liveRecord(m *Manager, mode tombstone.HolderMode) (uuid.UUID, tombstone.Record) {
	id := uuid.New()
	return id, tombstone.Record{
		OrchestrationID: id.String(),
		Path:            m.path.String(),
		Kind:            tombstone.KindDeleted,
		PurgedAt:        time.Date(2026, 11, 17, 15, 33, 47, 0, time.UTC),
		Holders:         map[string]tombstone.Holder{m.localhost: {State: tombstone.HolderPending, Mode: mode}},
	}
}

// A live holder of the node takes on the delete of the tombstone, its
// stamp and id, when no orchestration runs, and writes no tombstone of its
// own for it.
func TestAdoptTombstone(t *testing.T) {
	m, _, recorded := newTombstoneTestManager(t)
	m.state.GlobalExpect = instance.MonitorGlobalExpectNone
	m.state.OrchestrationID = uuid.Nil
	m.nodeMonitor[m.localhost] = node.Monitor{State: node.MonitorStateIdle}
	id, rec := liveRecord(m, tombstone.HolderLive)
	m.tombstones = map[string]tombstone.Record{id.String(): rec}

	m.state.GlobalExpect = instance.MonitorGlobalExpectStarted
	assert.False(t, m.adoptTombstone(), "another orchestration runs")
	m.state.GlobalExpect = instance.MonitorGlobalExpectNone

	m.nodeMonitor[m.localhost] = node.Monitor{State: node.MonitorStateRejoin}
	assert.False(t, m.adoptTombstone(), "not before the rejoin")
	m.nodeMonitor[m.localhost] = node.Monitor{State: node.MonitorStateIdle}

	require.True(t, m.adoptTombstone())
	assert.Equal(t, instance.MonitorGlobalExpectDeleted, m.state.GlobalExpect)
	assert.Equal(t, id, m.state.OrchestrationID)
	assert.True(t, rec.PurgedAt.Equal(m.state.GlobalExpectUpdatedAt))
	require.NoError(t, m.recordTombstone())
	assert.Empty(t, *recorded, "the record names the holders already")
}

// An away holder never deletes on its own.
func TestAnAwayHolderAdoptsNothing(t *testing.T) {
	m, _, _ := newTombstoneTestManager(t)
	m.state.GlobalExpect = instance.MonitorGlobalExpectNone
	m.state.OrchestrationID = uuid.Nil
	m.nodeMonitor[m.localhost] = node.Monitor{State: node.MonitorStateIdle}
	id, rec := liveRecord(m, tombstone.HolderAway)
	m.tombstones = map[string]tombstone.Record{id.String(): rec}
	assert.False(t, m.adoptTombstone())
	assert.Equal(t, instance.MonitorGlobalExpectNone, m.state.GlobalExpect)
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

// An away holder shows the issue, is flagged stopped once, and refuses what
// would start or provision it; the issue goes with the holder cleared.
func TestAwayHolderIssue(t *testing.T) {
	m, pub, _ := newTombstoneTestManager(t)
	rawVar := rawconfig.Paths.Var
	rawconfig.Paths.Var = t.TempDir()
	t.Cleanup(func() { rawconfig.Paths.Var = rawVar })
	id, rec := liveRecord(m, tombstone.HolderAway)
	m.tombstones = map[string]tombstone.Record{id.String(): rec}
	m.applyTombstoneIssue()
	require.Len(t, m.state.Issues, 1)
	assert.Contains(t, m.state.Issues[0], "while this node was away")
	assert.Empty(t, m.tombstoneStopped, "no status known yet: the flag waits")
	m.instStatus[m.localhost] = instance.Status{Avail: status.Down, UpdatedAt: time.Now()}
	m.applyTombstoneIssue()
	stopped := func() int {
		n := 0
		for _, msg := range pub.msgs {
			if _, ok := msg.(*msgbus.InstanceStoppedFileUpdated); ok {
				n++
			}
		}
		return n
	}
	assert.Equal(t, 1, stopped())
	m.applyTombstoneIssue()
	assert.Equal(t, 1, stopped(), "flagged once per record")

	ok, reason := m.isRequestedStartable()
	assert.False(t, ok)
	assert.Contains(t, reason, "while this node was away")
	assert.True(t, refusedByTombstone(instance.MonitorGlobalExpectProvisioned))
	assert.False(t, refusedByTombstone(instance.MonitorGlobalExpectPurged))
	assert.False(t, refusedByTombstone(instance.MonitorGlobalExpectStopped))

	rec.Holders[m.localhost] = tombstone.Holder{State: tombstone.HolderCleared, Mode: tombstone.HolderAway, ClearedBy: tombstone.ClearedByKept}
	m.tombstones = map[string]tombstone.Record{id.String(): rec}
	m.applyTombstoneIssue()
	assert.Empty(t, m.state.Issues)
}

// An away holder whose instance is up is not flagged stopped: never stopped
// by the tombstone, the operator decides.
func TestAwayHolderUpNotFlagged(t *testing.T) {
	m, pub, _ := newTombstoneTestManager(t)
	m.instStatus[m.localhost] = instance.Status{Avail: status.Up, UpdatedAt: time.Now()}
	id, rec := liveRecord(m, tombstone.HolderAway)
	m.tombstones = map[string]tombstone.Record{id.String(): rec}
	m.applyTombstoneIssue()
	assert.Len(t, m.state.Issues, 1)
	for _, msg := range pub.msgs {
		_, ok := msg.(*msgbus.InstanceStoppedFileUpdated)
		assert.False(t, ok)
	}
}

// A live holder of the node whose delete fails, the peers done, turns away,
// once: the operator decides, the issue shown.
func TestLiveHolderFailedTurnsAway(t *testing.T) {
	m, _, _ := newTombstoneTestManager(t)
	var demoted []string
	saved := demoteTombstoneFunc
	demoteTombstoneFunc = func(path, _ string) error {
		demoted = append(demoted, path)
		return nil
	}
	t.Cleanup(func() { demoteTombstoneFunc = saved })
	_, rec := liveRecord(m, tombstone.HolderLive)
	rec.OrchestrationID = m.state.OrchestrationID.String()
	m.tombstones = map[string]tombstone.Record{rec.OrchestrationID: rec}

	m.state.State = instance.MonitorStateDeleteProgress
	m.demoteTombstoneIfFailed()
	assert.Empty(t, demoted, "no failure")
	m.state.State = instance.MonitorStateDeleteFailure
	m.demoteTombstoneIfFailed()
	assert.Equal(t, []string{m.path.String()}, demoted)
	m.refreshTombstones()
	assert.Len(t, demoted, 1, "once per orchestration")
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

// A node upgrading its packages, its daemon restarting, is an away holder,
// and its instance up refuses a purge without --force.
func TestUpgradingNodeIsAway(t *testing.T) {
	m, _, recorded := newTombstoneTestManager(t)
	m.nodeMonitor["node2"] = node.Monitor{State: node.MonitorStateUpgrade}
	require.NoError(t, m.recordTombstone())
	require.Len(t, *recorded, 1)
	assert.Equal(t, tombstone.HolderAway, (*recorded)[0].Holders["node2"].Mode)
	m.instStatus["node2"] = instance.Status{Avail: status.Up, Provisioned: provisioned.True}
	assert.Contains(t, m.awayInstancesRefusal(instance.MonitorGlobalExpectPurged), "node2")
}

// A live holder never adopts an orchestration its instance ran already, as
// one that failed and was aborted: it turns away once, the operator deciding
// (qatests 308, the aborted purge run again).
func TestNoAdoptionOfAnOrchestrationRunHere(t *testing.T) {
	m, _, _ := newTombstoneTestManager(t)
	var demoted []string
	saved := demoteTombstoneFunc
	demoteTombstoneFunc = func(path, _ string) error {
		demoted = append(demoted, path)
		return nil
	}
	t.Cleanup(func() { demoteTombstoneFunc = saved })
	m.state.GlobalExpect = instance.MonitorGlobalExpectNone
	m.state.OrchestrationID = uuid.Nil
	m.nodeMonitor[m.localhost] = node.Monitor{State: node.MonitorStateIdle}
	id, rec := liveRecord(m, tombstone.HolderLive)
	m.tombstones = map[string]tombstone.Record{id.String(): rec}
	m.carriedOrchestrationID = id

	assert.False(t, m.adoptTombstone(), "ran here already")
	assert.Equal(t, instance.MonitorGlobalExpectNone, m.state.GlobalExpect)
	assert.Equal(t, []string{m.path.String()}, demoted)
	assert.False(t, m.adoptTombstone())
	assert.Len(t, demoted, 1, "turned away once")
}

// The failure of a delete set by the end of its action, not by
// transitionTo, turns the live holder away too.
func TestLiveHolderFailedOnActionResult(t *testing.T) {
	m, _, _ := newTombstoneTestManager(t)
	var demoted []string
	saved := demoteTombstoneFunc
	demoteTombstoneFunc = func(path, _ string) error {
		demoted = append(demoted, path)
		return nil
	}
	t.Cleanup(func() { demoteTombstoneFunc = saved })
	_, rec := liveRecord(m, tombstone.HolderLive)
	rec.OrchestrationID = m.state.OrchestrationID.String()
	m.tombstones = map[string]tombstone.Record{rec.OrchestrationID: rec}
	m.state.State = instance.MonitorStateDeleteProgress
	assert.False(t, m.applyActionResult(cmdOrchestrate{state: instance.MonitorStateStartProgress, newState: instance.MonitorStateStartFailure}), "not the state the action ran in")
	assert.Empty(t, demoted)
	assert.True(t, m.applyActionResult(cmdOrchestrate{state: instance.MonitorStateDeleteProgress, newState: instance.MonitorStateDeleteFailure}))
	assert.Equal(t, []string{m.path.String()}, demoted)
}

// A stopped flag not written is not taken as decided: it is written at the
// next status or tombstone event.
func TestAwayHolderStoppedFlagRetried(t *testing.T) {
	m, _, _ := newTombstoneTestManager(t)
	rawVar := rawconfig.Paths.Var
	t.Cleanup(func() { rawconfig.Paths.Var = rawVar })
	// A file where the var directory is to be: no flag can be written.
	notADir := filepath.Join(t.TempDir(), "var")
	require.NoError(t, os.WriteFile(notADir, nil, 0o600))
	rawconfig.Paths.Var = notADir
	m.instStatus[m.localhost] = instance.Status{Avail: status.Down, UpdatedAt: time.Now()}
	id, rec := liveRecord(m, tombstone.HolderAway)
	m.tombstones = map[string]tombstone.Record{id.String(): rec}
	m.applyTombstoneIssue()
	assert.Empty(t, m.tombstoneStopped, "the write failed")

	rawconfig.Paths.Var = t.TempDir()
	m.applyTombstoneIssue()
	assert.Equal(t, id.String(), m.tombstoneStopped)
	assert.True(t, m.isStopped())
}

// A delete going on after a stop failed, as from idle, does not turn the live
// holder away: only the failures ending the orchestration do.
func TestLiveHolderNotTurnedAwayOnARecoverableFailure(t *testing.T) {
	m, _, _ := newTombstoneTestManager(t)
	var demoted []string
	saved := demoteTombstoneFunc
	demoteTombstoneFunc = func(path, id string) error {
		demoted = append(demoted, id)
		return nil
	}
	t.Cleanup(func() { demoteTombstoneFunc = saved })
	_, rec := liveRecord(m, tombstone.HolderLive)
	rec.OrchestrationID = m.state.OrchestrationID.String()
	m.tombstones = map[string]tombstone.Record{rec.OrchestrationID: rec}
	m.state.State = instance.MonitorStateStopFailure
	m.demoteTombstoneIfFailed()
	assert.Empty(t, demoted, "a delete goes on after a stop failure")
	m.state.State = instance.MonitorStateDeleteFailure
	m.demoteTombstoneIfFailed()
	assert.Equal(t, []string{rec.OrchestrationID}, demoted, "with the id of the failed orchestration")
}
