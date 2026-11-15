package imon

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
)

// newProvisionAffinityTestManager returns the manager of node1, leading the
// placement of a failover object provisioned nowhere, of the affinities
// given, a provisioned orchestration running.
func newProvisionAffinityTestManager(cfg instance.ActorConfig) *Manager {
	m := newAffinityTestManager(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.objStatus.Topology = topology.Failover
	m.state.IsLeader = true
	m.state.GlobalExpect = instance.MonitorGlobalExpectProvisioned
	m.state.OrchestrationID = uuid.New()
	m.waitAffinityTimer = time.NewTimer(time.Hour)
	m.waitAffinityTimer.Stop()
	delete(m.instMonitor, m.localhost)
	for _, n := range m.scopeNodes {
		m.instStatus[n] = instance.Status{Avail: status.Down, Provisioned: provisioned.False}
		if n != m.localhost {
			m.instMonitor[n] = instance.Monitor{State: instance.MonitorStateIdle, OrchestrationID: m.state.OrchestrationID}
		}
	}
	return m
}

// setAffineMonitor sets the monitor of the instance of p on node.
func setAffineMonitor(t *testing.T, p naming.Path, node string, mon instance.Monitor) {
	instance.MonitorData.Set(p, node, &mon)
	t.Cleanup(func() { instance.MonitorData.Unset(p, node) })
}

// A provision starts the object where a start would: on the first node its
// hard affinities let it start on, else on the first node its soft
// affinities prefer, not on the placement leader. It used to start it on the
// placement leader, whatever its affinities (card 302).
func TestAProvisionIsLedFromWhereTheAffinitiesLetTheObjectStart(t *testing.T) {
	affine := naming.Path{Kind: naming.KindSvc, Name: "affine-302"}
	m := newProvisionAffinityTestManager(instance.ActorConfig{HardAffinity: naming.Paths{affine}})
	setAffine(t, affine, map[string]status.T{"node1": status.Down, "node2": status.Up, "node3": status.Down})
	assert.Equal(t, "node2", m.provisioningLeader(), "hard affinity")
	assert.False(t, m.isProvisioningLeader())

	m = newProvisionAffinityTestManager(instance.ActorConfig{SoftAffinity: naming.Paths{affine}})
	assert.Equal(t, "node2", m.provisioningLeader(), "soft affinity")

	m = newProvisionAffinityTestManager(instance.ActorConfig{SoftAntiAffinity: naming.Paths{affine}})
	assert.Equal(t, "node1", m.provisioningLeader(), "soft anti affinity on node2 only")

	m = newProvisionAffinityTestManager(instance.ActorConfig{})
	assert.Equal(t, "node1", m.provisioningLeader(), "no affinity: the placement leader")

	m = newProvisionAffinityTestManager(instance.ActorConfig{HardAffinity: naming.Paths{affine}})
	m.state.GlobalExpect = instance.MonitorGlobalExpectUnprovisioned
	assert.Equal(t, "node1", m.provisioningLeader(), "the unprovision leader is the placement leader")
}

// No node satisfying the hard affinities, the placement leader waits for an
// object they name to come up while it shows it does, and the leadership
// moves to where it came up.
func TestAProvisionWaitsForAnAffineObjectComingUp(t *testing.T) {
	affine := naming.Path{Kind: naming.KindSvc, Name: "affine-302-wait"}
	m := newProvisionAffinityTestManager(instance.ActorConfig{HardAffinity: naming.Paths{affine}})
	setAffine(t, affine, map[string]status.T{"node1": status.Down, "node2": status.Down})
	setAffineMonitor(t, affine, "node2", instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectProvisioned, State: instance.MonitorStateProvisionProgress})
	queued := queuedActions(t)

	require.Equal(t, "node1", m.provisioningLeader())
	m.orchestrateProvisioned()
	assert.Equal(t, instance.MonitorStateWaitAffinity, m.state.State)
	assert.Empty(t, *queued)
	m.orchestrateProvisioned()
	assert.Equal(t, instance.MonitorStateWaitAffinity, m.state.State, "the affine object is coming up")

	setAffine(t, affine, map[string]status.T{"node1": status.Down, "node2": status.Up})
	require.Equal(t, "node2", m.provisioningLeader())
	m.orchestrateProvisioned()
	assert.Equal(t, instance.MonitorStateIdle, m.state.State, "the leadership moved to node2")
}

// The wait ends on a failure, with the reason the client prints, when an
// object waited for failed to come up.
func TestAProvisionWaitFailsWhenTheAffineObjectFailed(t *testing.T) {
	affine := naming.Path{Kind: naming.KindSvc, Name: "affine-302-fail"}
	m := newProvisionAffinityTestManager(instance.ActorConfig{HardAffinity: naming.Paths{affine}})
	setAffine(t, affine, map[string]status.T{"node1": status.Down, "node2": status.Down})
	setAffineMonitor(t, affine, "node2", instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectStarted, State: instance.MonitorStateStartProgress})
	m.orchestrateProvisioned()
	require.Equal(t, instance.MonitorStateWaitAffinity, m.state.State)

	setAffineMonitor(t, affine, "node2", instance.Monitor{
		GlobalExpect:        instance.MonitorGlobalExpectStarted,
		OrchestrationIsDone: true,
		State:               instance.MonitorStateStartFailure,
		StateUpdatedAt:      time.Now(),
	})
	m.orchestrateProvisioned()
	assert.Equal(t, instance.MonitorStateWaitAffinityFailure, m.state.State)
	assert.Equal(t, "hard affinity with affine-302-fail: start failed on node2", m.state.StateReason)
	m.orchestrateProvisioned()
	assert.True(t, m.state.OrchestrationIsDone, "the orchestration ends on the failure")
	assert.Equal(t, instance.MonitorStateWaitAffinityFailure, m.state.State, "the failure lingers")

	// A new orchestration starts from it as from idle.
	m.state.OrchestrationID = uuid.New()
	m.state.OrchestrationIsDone = false
	setAffine(t, affine, map[string]status.T{"node1": status.Up})
	queued := queuedActions(t)
	m.orchestrateProvisioned()
	assert.Equal(t, []instance.MonitorState{instance.MonitorStatePreflightProgress}, *queued)
	assert.Empty(t, m.state.StateReason)
}

// The wait ends on a failure when an object waited for shows no intention
// to come up for waitAffinityGrace.
func TestAProvisionWaitFailsWhenTheAffineObjectIsNotComingUp(t *testing.T) {
	saved := waitAffinityGrace
	waitAffinityGrace = 50 * time.Millisecond
	t.Cleanup(func() { waitAffinityGrace = saved })
	affine := naming.Path{Kind: naming.KindSvc, Name: "affine-302-idle"}
	m := newProvisionAffinityTestManager(instance.ActorConfig{HardAffinity: naming.Paths{affine}})
	setAffine(t, affine, map[string]status.T{"node1": status.Down, "node2": status.Down})
	setAffineMonitor(t, affine, "node2", instance.Monitor{State: instance.MonitorStateIdle})
	m.orchestrateProvisioned()
	require.Equal(t, instance.MonitorStateWaitAffinity, m.state.State, "within the grace")
	time.Sleep(60 * time.Millisecond)
	m.orchestrateProvisioned()
	assert.Equal(t, instance.MonitorStateWaitAffinityFailure, m.state.State)
	assert.Contains(t, m.state.StateReason, "not up, and not coming up")
}

// A provision the hard affinities can never let start is refused: an object
// they name does not exist, or has no instance on the nodes of the object, or
// the hard anti affinities refuse every node.
func TestAProvisionNeverMetIsRefused(t *testing.T) {
	missing := naming.Path{Kind: naming.KindSvc, Name: "affine-302-missing"}
	m := newProvisionAffinityTestManager(instance.ActorConfig{HardAffinity: naming.Paths{missing}})
	assert.Equal(t, "hard affinity with affine-302-missing: no such object", m.hardAffinityNeverMet(m.scopeNodes))

	other := naming.Path{Kind: naming.KindSvc, Name: "affine-302-elsewhere"}
	instance.ConfigData.Set(other, "node9", &instance.Config{Scope: []string{"node9"}, ActorConfig: &instance.ActorConfig{}})
	t.Cleanup(func() { instance.ConfigData.Unset(other, "node9") })
	m = newProvisionAffinityTestManager(instance.ActorConfig{HardAffinity: naming.Paths{other}})
	assert.Contains(t, m.hardAffinityNeverMet(m.scopeNodes), "it has no instance on node1, node2, node3")

	anti := naming.Path{Kind: naming.KindSvc, Name: "affine-302-anti"}
	setAffine(t, anti, map[string]status.T{"node1": status.Up, "node2": status.Up, "node3": status.Up})
	m = newProvisionAffinityTestManager(instance.ActorConfig{HardAntiAffinity: naming.Paths{anti}})
	assert.Contains(t, m.hardAffinityNeverMet(m.scopeNodes), "hard anti affinity with affine-302-anti not satisfied on node1")

	setAffine(t, anti, map[string]status.T{"node1": status.Up, "node2": status.Down})
	assert.Equal(t, "", m.hardAffinityNeverMet(m.scopeNodes), "node2 and node3 not refused")
}

// newStartAffinityTestManager returns the manager of node1, of a failover
// object provisioned and down on node1, node2 and node3, of the affinities
// given, a start a user asked for running.
func newStartAffinityTestManager(cfg instance.ActorConfig) *Manager {
	m := newProvisionAffinityTestManager(cfg)
	m.state.GlobalExpect = instance.MonitorGlobalExpectStarted
	for _, n := range m.scopeNodes {
		m.instStatus[n] = instance.Status{Avail: status.Down, Provisioned: provisioned.True}
	}
	return m
}

// A start no node can do yet, an object of the hard affinities down
// everywhere, waits on the first candidate for that object to come up, and
// goes where it came up. It used to be refused, which a request starting both
// objects hit.
func TestAStartWaitsForAnAffineObjectComingUp(t *testing.T) {
	affine := naming.Path{Kind: naming.KindSvc, Name: "affine-start"}
	m := newStartAffinityTestManager(instance.ActorConfig{HardAffinity: naming.Paths{affine}})
	setAffine(t, affine, map[string]status.T{"node1": status.Down, "node2": status.Down})
	setAffineMonitor(t, affine, "node2", instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectStarted, State: instance.MonitorStateReady})
	ok, reason := m.isRequestedStartable()
	require.True(t, ok, reason)

	require.Equal(t, "node1", m.startAffinityWaiter())
	m.orchestrateStarted()
	assert.Equal(t, instance.MonitorStateWaitAffinity, m.state.State)

	setAffine(t, affine, map[string]status.T{"node1": status.Down, "node2": status.Up})
	assert.Equal(t, "", m.startAffinityWaiter(), "node2 satisfies the hard affinity")
	m.orchestrateStarted()
	assert.Equal(t, instance.MonitorStateIdle, m.state.State)
	assert.False(t, m.isStartLeader(), "node2 starts, not node1")
}

// A start waiting for an object that failed to come up fails, and the peers
// end the orchestration too.
func TestAStartWaitFailsAndThePeersEnd(t *testing.T) {
	affine := naming.Path{Kind: naming.KindSvc, Name: "affine-start-fail"}
	m := newStartAffinityTestManager(instance.ActorConfig{HardAffinity: naming.Paths{affine}})
	setAffine(t, affine, map[string]status.T{"node1": status.Down, "node2": status.Down})
	setAffineMonitor(t, affine, "node2", instance.Monitor{State: instance.MonitorStateStartFailure, StateUpdatedAt: time.Now().Add(time.Second)})
	m.orchestrateStarted()
	assert.Equal(t, instance.MonitorStateWaitAffinityFailure, m.state.State)
	assert.Equal(t, "hard affinity with affine-start-fail: start failed on node2", m.state.StateReason)
	m.orchestrateStarted()
	assert.True(t, m.state.OrchestrationIsDone)

	peer := newStartAffinityTestManager(instance.ActorConfig{HardAffinity: naming.Paths{affine}})
	peer.localhost = "node2"
	peer.state.OrchestrationID = m.state.OrchestrationID
	peer.instMonitor = map[string]instance.Monitor{"node1": *m.state.DeepCopy()}
	peer.orchestrateStarted()
	assert.True(t, peer.state.OrchestrationIsDone, "the peer ends the orchestration")
}

// The reason of a failure is cleared once the state is no failure, the state
// set directly, as by a clear of the monitor, or by a transition, and kept
// while it is.
func TestTheReasonOfAFailureIsClearedWithIt(t *testing.T) {
	m := newProvisionAffinityTestManager(instance.ActorConfig{})
	m.state.State = instance.MonitorStateWaitAffinityFailure
	m.state.StateReason = "hard affinity with p: start failed on node2"
	m.clearStaleStateReason()
	assert.NotEmpty(t, m.state.StateReason, "kept while failed")

	m.state.State = instance.MonitorStateIdle
	m.clearStaleStateReason()
	assert.Empty(t, m.state.StateReason, "cleared by a state set directly")

	m.state.StateReason = "stale"
	m.transitionTo(instance.MonitorStateIdle)
	assert.Empty(t, m.state.StateReason, "cleared by a transition")
}
