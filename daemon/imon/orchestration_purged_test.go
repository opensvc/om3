package imon

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
)

// A purge unprovisions a mixed instance, as one whose provision failed half
// way, before deleting it: what its provisioned resources took, as a drbd
// resource up on the disk of a volume, would otherwise outlive the object.
// Here the local instance leads the unprovision, so it waits for the peer
// still provisioned, rather than going straight to the delete.
func TestAPurgeUnprovisionsAMixedInstance(t *testing.T) {
	m := newTestManager(&pubSpy{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.path = naming.Path{Kind: naming.KindVol, Name: "obj"}
	m.state.GlobalExpect = instance.MonitorGlobalExpectPurged
	m.state.OrchestrationID = uuid.New()
	m.instStatus[m.localhost] = instance.Status{
		Avail:       status.Warn,
		Provisioned: provisioned.Mixed,
	}
	m.instStatus["node2"] = instance.Status{
		Avail:       status.Down,
		Provisioned: provisioned.True,
	}

	m.orchestratePurged()
	require.Equal(t, instance.MonitorStateWaitNonLeader, m.state.State,
		"the mixed instance must be unprovisioned, after its peers, not deleted as it is")
}

// A purge goes on from a start failed, as one of the restart of the standby
// resources of a booting node: there is nothing left to keep, and the purge
// unprovisions the instance as from idle.
func TestAPurgeGoesOnFromAStartFailed(t *testing.T) {
	for _, state := range []instance.MonitorState{
		instance.MonitorStateStartFailure,
		instance.MonitorStateProvisionFailure,
		instance.MonitorStateBootFailed,
	} {
		m := newTestManager(&pubSpy{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		m.ctx = ctx
		m.path = naming.Path{Kind: naming.KindVol, Name: "obj"}
		m.state.State = state
		m.state.GlobalExpect = instance.MonitorGlobalExpectPurged
		m.state.OrchestrationID = uuid.New()
		m.instStatus[m.localhost] = instance.Status{Avail: status.Warn, Provisioned: provisioned.True}
		m.instStatus["node2"] = instance.Status{Avail: status.Down, Provisioned: provisioned.True}

		m.orchestratePurged()
		require.Equal(t, instance.MonitorStateWaitNonLeader, m.state.State, "from %s", state)
	}
}

// A purge unprovisions an instance read as not provisioned too, as in v2: a
// resource with no provisioned state, as the container of an oci resource,
// may hold what only its unprovision releases while the others read false.
func TestAPurgeUnprovisionsAnInstanceReadNotProvisioned(t *testing.T) {
	m := newTestManager(&pubSpy{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.path = naming.Path{Kind: naming.KindSvc, Name: "obj"}
	m.state.GlobalExpect = instance.MonitorGlobalExpectPurged
	m.state.OrchestrationID = uuid.New()
	m.instStatus[m.localhost] = instance.Status{Avail: status.Warn, Provisioned: provisioned.False}
	m.instStatus["node2"] = instance.Status{Avail: status.Down, Provisioned: provisioned.True}

	m.orchestratePurged()
	require.Equal(t, instance.MonitorStateWaitNonLeader, m.state.State,
		"the instance read not provisioned must be unprovisioned, not deleted as it is")
}

// newPurgeLeaderWaiting returns a manager leading the unprovision of a purge,
// waiting for its non-leaders.
func newPurgeLeaderWaiting() *Manager {
	m := newTestManager(&pubSpy{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.path = naming.Path{Kind: naming.KindSvc, Name: "obj"}
	m.state.State = instance.MonitorStateWaitNonLeader
	m.state.GlobalExpect = instance.MonitorGlobalExpectPurged
	m.state.OrchestrationID = uuid.New()
	// An instance running, warn here, leads the unprovision.
	m.instStatus[m.localhost] = instance.Status{Avail: status.Warn, Provisioned: provisioned.True}
	return m
}

// A non-leader whose unprovision failed stays provisioned, and is not
// retried: the leader gives up rather than wait for it forever, and ends in
// purge failed, so the orchestration ends. It does not unprovision its
// instance, whose unprovision releases what the non-leader still holds.
func TestAPurgeLeaderGivesUpOnANonLeaderFailedProvisioned(t *testing.T) {
	m := newPurgeLeaderWaiting()
	m.instStatus["node2"] = instance.Status{Avail: status.Down, Provisioned: provisioned.Mixed}
	m.instMonitor["node2"] = instance.Monitor{State: instance.MonitorStatePurgeFailed, OrchestrationIsDone: true}

	m.orchestratePurged()
	require.Equal(t, instance.MonitorStatePurgeFailed, m.state.State, "the leader must give up")

	m.orchestratePurged()
	require.True(t, m.state.OrchestrationIsDone, "the orchestration must be marked done")
	require.Equal(t, instance.MonitorStatePurgeFailed, m.state.State,
		"the failure must linger, so an operator can see what failed")
}

// A non-leader whose purge failed once it was not provisioned any more, as
// on its delete, does not hold the leader, which goes on to its unprovision.
// Nor does it make the leader give up on another non-leader still
// unprovisioning: the leader waits for that one.
func TestAPurgeLeaderGoesOnPastANonLeaderFailedUnprovisioned(t *testing.T) {
	m := newPurgeLeaderWaiting()
	m.instStatus["node2"] = instance.Status{Avail: status.Down, Provisioned: provisioned.False}
	m.instMonitor["node2"] = instance.Monitor{State: instance.MonitorStatePurgeFailed, OrchestrationIsDone: true}
	m.instStatus["node3"] = instance.Status{Avail: status.Down, Provisioned: provisioned.True}
	m.instMonitor["node3"] = instance.Monitor{State: instance.MonitorStateUnprovisionProgress}

	m.orchestratePurged()
	require.Equal(t, instance.MonitorStateWaitNonLeader, m.state.State,
		"the leader must wait for the non-leader still unprovisioning")
	require.False(t, m.state.OrchestrationIsDone)

	queued := make([]instance.MonitorState, 0)
	testQueueAction = func(newState instance.MonitorState) {
		queued = append(queued, newState)
	}
	t.Cleanup(func() { testQueueAction = nil })
	m.instStatus["node3"] = instance.Status{Avail: status.Down, Provisioned: provisioned.False}
	m.instMonitor["node3"] = instance.Monitor{State: instance.MonitorStateIdle, OrchestrationIsDone: true}
	m.orchestratePurged()
	require.Equal(t, []instance.MonitorState{instance.MonitorStateUnprovisionProgress}, queued,
		"a non-leader failed but not provisioned must not stop the leader, which unprovisions")
}
