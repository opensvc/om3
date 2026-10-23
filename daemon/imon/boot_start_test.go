package imon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
)

// newBootStartTestManager returns the manager of the placement leader of an
// orchestrate=start object, on the daemon start following its node boot, its
// instance down, and the peer instance down and in the state given.
func newBootStartTestManager(peerState instance.MonitorState) *Manager {
	m := newTestManager(&pubSpy{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.isBootStartPending = true
	m.state.IsLeader = true
	m.objStatus.Orchestrate = "start"
	m.instConfig.Scope = []string{"node1", "node2"}
	m.nodeStatus = map[string]node.Status{"node1": {}, "node2": {}}
	m.instStatus["node1"] = instance.Status{Avail: status.StandbyDown, Provisioned: provisioned.True, UpdatedAt: time.Now()}
	m.instStatus["node2"] = instance.Status{Avail: status.StandbyDown, Provisioned: provisioned.True, UpdatedAt: time.Now()}
	m.instMonitor["node2"] = instance.Monitor{State: peerState}
	return m
}

// A peer starting its standby resources at its boot, as every node does, is
// not a reason to start nothing: the decision waits for it to end, and the
// start is readied then.
func TestBootStartWaitsForAPeerStartingItsStandbyResources(t *testing.T) {
	m := newBootStartTestManager(instance.MonitorStateStartProgress)
	m.orchestrateBootStart()
	require.True(t, m.isBootStartPending, "the decision waits")
	require.Equal(t, instance.MonitorStateIdle, m.state.State)

	m.instMonitor["node2"] = instance.Monitor{State: instance.MonitorStateIdle}
	m.orchestrateBootStart()
	require.Equal(t, instance.MonitorStateReady, m.state.State, "the start is readied once the peer is idle")
}

// A peer busy past the bound closes the decision: a start long after the
// node came up is not what orchestrate=start says.
func TestBootStartWaitIsBounded(t *testing.T) {
	m := newBootStartTestManager(instance.MonitorStateStartProgress)
	m.orchestrateBootStart()
	m.bootStartWaitSince = time.Now().Add(-bootStartWaitMax - time.Second)
	m.instMonitor["node2"] = instance.Monitor{State: instance.MonitorStateIdle}
	m.orchestrateBootStart()
	require.False(t, m.isBootStartPending)
	require.Equal(t, instance.MonitorStateIdle, m.state.State, "nothing started")
}

// A start the rules do not call for closes the decision, saying why.
func TestBootStartSaysWhyNothingIsStarted(t *testing.T) {
	m := newBootStartTestManager(instance.MonitorStateIdle)
	m.state.IsLeader = false
	m.orchestrateBootStart()
	require.False(t, m.isBootStartPending)
	require.Equal(t, "this node is not the placement leader", m.bootStartNoopReason())
}

// A resource stopped on purpose keeps the boot of an orchestrate=start object
// from starting its instance, which would start the resource back: the
// decision closes, saying which resources.
func TestBootStartLeavesAResourceStoppedOnPurpose(t *testing.T) {
	m := newBootStartTestManager(instance.MonitorStateIdle)
	local := m.instStatus["node1"]
	local.Resources = instance.ResourceStatuses{"app#1": resource.Status{IsStopped: true}}
	m.instStatus["node1"] = local
	m.orchestrateBootStart()
	require.False(t, m.isBootStartPending)
	require.Equal(t, instance.MonitorStateIdle, m.state.State, "nothing started")
	require.Equal(t, "the resources app#1 are flagged stopped", m.bootStartNoopReason())
}
