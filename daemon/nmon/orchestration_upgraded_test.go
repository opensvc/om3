package nmon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

func idle(expect node.MonitorGlobalExpect) node.Monitor {
	return node.Monitor{State: node.MonitorStateIdle, GlobalExpect: expect}
}

// The nodes upgrade one at a time, in the order of the cluster nodes, each
// once every node is live and idle, and none once one failed.
func TestUpgradeTurnOf(t *testing.T) {
	upgraded := node.MonitorGlobalExpectUpgraded
	none := node.MonitorGlobalExpectNone
	for _, tc := range []struct {
		name     string
		nodes    []string
		monitors map[string]node.Monitor
		want     upgradeTurn
	}{
		{
			"the first expecting node goes first",
			[]string{"n1", "n2", "n3"},
			map[string]node.Monitor{"n1": idle(upgraded), "n2": idle(upgraded), "n3": idle(upgraded)},
			upgradeTurn{node: "n1"},
		},
		{
			"in the order of the cluster nodes",
			[]string{"n3", "n1", "n2"},
			map[string]node.Monitor{"n1": idle(upgraded), "n2": idle(upgraded), "n3": idle(upgraded)},
			upgradeTurn{node: "n3"},
		},
		{
			"a node done is passed over",
			[]string{"n1", "n2", "n3"},
			map[string]node.Monitor{"n1": idle(none), "n2": idle(upgraded), "n3": idle(upgraded)},
			upgradeTurn{node: "n2"},
		},
		{
			"a node upgrading holds the others",
			[]string{"n1", "n2", "n3"},
			map[string]node.Monitor{"n1": {State: node.MonitorStateUpgrade, GlobalExpect: upgraded}, "n2": idle(upgraded), "n3": idle(upgraded)},
			upgradeTurn{wait: "node n1 is upgrade"},
		},
		{
			"a node restarting holds the others",
			[]string{"n1", "n2", "n3"},
			map[string]node.Monitor{"n2": idle(upgraded), "n3": idle(upgraded)},
			upgradeTurn{wait: "node n1 is not live"},
		},
		{
			"a node rejoining holds the others",
			[]string{"n1", "n2", "n3"},
			map[string]node.Monitor{"n1": {State: node.MonitorStateRejoin}, "n2": idle(upgraded), "n3": idle(upgraded)},
			upgradeTurn{wait: "node n1 is rejoin"},
		},
		{
			"a node whose upgrade failed fails the upgrade",
			[]string{"n1", "n2", "n3"},
			map[string]node.Monitor{"n1": {State: node.MonitorStateUpgradeFailure}, "n2": idle(upgraded), "n3": idle(upgraded)},
			upgradeTurn{failed: "upgrade failed on n1"},
		},
		{
			"a failure wins over a wait",
			[]string{"n1", "n2", "n3"},
			map[string]node.Monitor{"n2": idle(upgraded), "n3": {State: node.MonitorStateFreezeFailure}},
			upgradeTurn{failed: "freeze failed on n3"},
		},
		{
			"the cluster nodes unknown, the monitors say them",
			nil,
			map[string]node.Monitor{"n2": idle(upgraded), "n1": idle(upgraded)},
			upgradeTurn{node: "n1"},
		},
		{
			"nobody expects the upgrade",
			[]string{"n1", "n2"},
			map[string]node.Monitor{"n1": idle(none), "n2": idle(none)},
			upgradeTurn{wait: "no node expects the upgrade"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, upgradeTurnOf(tc.nodes, tc.monitors))
		})
	}
}

// newUpgradeTestManager is a manager expecting the upgrade of a two-node
// cluster, its peer idle and expecting it too, with a var directory of its
// own for the upgrade flag.
func newUpgradeTestManager(t *testing.T, publisher *recordingPublisher) (*Manager, uuid.UUID) {
	t.Helper()
	rawconfig.Paths.Var = t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(rawconfig.Paths.Var, "node"), 0o755))
	m := newTestManager(publisher)
	m.clusterConfig.Nodes = []string{"node1", "node2"}
	m.nodeMonitor = map[string]node.Monitor{"node2": idle(node.MonitorGlobalExpectUpgraded)}
	m.state.GlobalExpect = node.MonitorGlobalExpectUpgraded
	id := uuid.New()
	m.adoptOrchestration(id)
	return m, id
}

func lastEnd(t *testing.T, publisher *recordingPublisher) *msgbus.NodeOrchestrationEnd {
	t.Helper()
	for i := len(publisher.messages) - 1; i >= 0; i-- {
		if end, ok := publisher.messages[i].(*msgbus.NodeOrchestrationEnd); ok {
			return end
		}
	}
	t.Fatalf("no orchestration end published: %v", kinds(publisher.messages))
	return nil
}

// A daemon restarted by its upgrade adopts the orchestration again, and ends
// its part on the flag it wrote before upgrading, running nothing.
func TestARestartedDaemonEndsItsUpgradeByTheFlag(t *testing.T) {
	publisher := &recordingPublisher{}
	m, id := newUpgradeTestManager(t, publisher)
	require.NoError(t, m.writeUpgradeFlag(id.String()))

	m.orchestrateUpgraded()

	require.Equal(t, node.MonitorGlobalExpectNone, m.state.GlobalExpect)
	require.Equal(t, node.MonitorStateIdle, m.state.State)
	require.Equal(t, uuid.Nil, m.state.OrchestrationID)
	require.NoFileExists(t, upgradeFlagFile())
	end := lastEnd(t, publisher)
	require.False(t, end.Failed)
	require.False(t, end.Aborted)
}

// A flag of another orchestration is one left behind: the node is not done.
func TestAFlagOfAnotherOrchestrationDoesNotEndTheUpgrade(t *testing.T) {
	publisher := &recordingPublisher{}
	m, _ := newUpgradeTestManager(t, publisher)
	require.NoError(t, m.writeUpgradeFlag(uuid.New().String()))
	// node2 goes first: this node waits, and runs nothing.
	m.clusterConfig.Nodes = []string{"node2", "node1"}

	m.orchestrateUpgraded()

	require.Equal(t, node.MonitorGlobalExpectUpgraded, m.state.GlobalExpect)
	require.Equal(t, node.MonitorStateIdle, m.state.State)
	require.Equal(t, "node node2 upgrades before this one", m.upgradeWaiting)
	require.Equal(t, []string{"NodeOrchestrationAccepted"}, kinds(publisher.messages))
}

// A node waits for the one before it in the order, then for it to be back
// and idle once its daemon restarts.
func TestAnUpgradeWaitsForThePeerToBeBack(t *testing.T) {
	publisher := &recordingPublisher{}
	m, _ := newUpgradeTestManager(t, publisher)
	m.clusterConfig.Nodes = []string{"node2", "node1"}

	m.nodeMonitor["node2"] = node.Monitor{State: node.MonitorStateUpgrade, GlobalExpect: node.MonitorGlobalExpectUpgraded}
	m.orchestrateUpgraded()
	require.Equal(t, "node node2 is upgrade", m.upgradeWaiting)

	delete(m.nodeMonitor, "node2")
	m.orchestrateUpgraded()
	require.Equal(t, "node node2 is not live", m.upgradeWaiting)

	m.nodeMonitor["node2"] = node.Monitor{State: node.MonitorStateRejoin, GlobalExpect: node.MonitorGlobalExpectUpgraded}
	m.orchestrateUpgraded()
	require.Equal(t, "node node2 is rejoin", m.upgradeWaiting)

	require.Equal(t, node.MonitorGlobalExpectUpgraded, m.state.GlobalExpect)
	require.Equal(t, node.MonitorStateIdle, m.state.State)
	require.Equal(t, []string{"NodeOrchestrationAccepted"}, kinds(publisher.messages))
}

// The upgrade is given up on a node in a failed state, this node ending its
// part as failed without running anything.
func TestAnUpgradeGivesUpOnAFailedPeer(t *testing.T) {
	publisher := &recordingPublisher{}
	m, _ := newUpgradeTestManager(t, publisher)
	m.nodeMonitor["node2"] = node.Monitor{State: node.MonitorStateUpgradeFailure}

	m.orchestrateUpgraded()

	require.Equal(t, node.MonitorGlobalExpectNone, m.state.GlobalExpect)
	require.Equal(t, node.MonitorStateIdle, m.state.State)
	require.Equal(t, uuid.Nil, m.state.OrchestrationID)
	end := lastEnd(t, publisher)
	require.True(t, end.Failed)
	require.Equal(t, "upgrade failed on node2", end.Error)
}

// An upgrade that did not restart the daemon ends from its success state,
// idle again; one that failed ends failed, keeping the state that says so.
func TestAnUpgradeEndsOnItsOutcome(t *testing.T) {
	for _, tc := range []struct {
		name      string
		state     node.MonitorState
		wantState node.MonitorState
		failed    bool
	}{
		{"upgraded", node.MonitorStateUpgradeSuccess, node.MonitorStateIdle, false},
		{"upgrade failed", node.MonitorStateUpgradeFailure, node.MonitorStateUpgradeFailure, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			publisher := &recordingPublisher{}
			m, id := newUpgradeTestManager(t, publisher)
			require.NoError(t, m.writeUpgradeFlag(id.String()))
			m.state.State = tc.state

			m.orchestrateUpgraded()

			require.Equal(t, node.MonitorGlobalExpectNone, m.state.GlobalExpect)
			require.Equal(t, tc.wantState, m.state.State)
			require.Equal(t, uuid.Nil, m.state.OrchestrationID)
			require.NoFileExists(t, upgradeFlagFile())
			require.Equal(t, tc.failed, lastEnd(t, publisher).Failed)
		})
	}
}

// With nothing expected, an upgraded state left by an abort settles to idle,
// and a flag left behind is removed once the node heard its peers.
func TestSettleUpgraded(t *testing.T) {
	publisher := &recordingPublisher{}
	m, id := newUpgradeTestManager(t, publisher)
	m.state.GlobalExpect = node.MonitorGlobalExpectNone
	require.NoError(t, m.writeUpgradeFlag(id.String()))

	m.state.State = node.MonitorStateUpgradeSuccess
	m.settleUpgraded()
	require.Equal(t, node.MonitorStateIdle, m.state.State)
	require.NoFileExists(t, upgradeFlagFile())

	require.NoError(t, m.writeUpgradeFlag(id.String()))
	m.settleUpgraded()
	require.FileExists(t, upgradeFlagFile(), "a daemon that heard no peer yet keeps the flag")

	m.state.GlobalExpectUpdatedAt = m.state.GlobalExpectUpdatedAt.Add(1)
	m.settleUpgraded()
	require.NoFileExists(t, upgradeFlagFile())
}
