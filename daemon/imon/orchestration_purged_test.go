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
