package imon

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/status"
)

// newPlacedAtTestManager returns the manager of the node an object is placed
// on, waiting in the stopped state for the source to hand it over, the
// source done with its part.
func newPlacedAtTestManager(objAvail, localAvail, peerAvail status.T) *Manager {
	m := newTestManager(&pubSpy{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.state.State = instance.MonitorStateStopSuccess
	m.state.GlobalExpect = instance.MonitorGlobalExpectPlacedAt
	m.state.OrchestrationID = uuid.New()
	m.objStatus.Avail = objAvail
	m.waitConvergedOrchestrationMsg = make(map[string]string)
	m.instStatus[m.localhost] = instance.Status{Avail: localAvail}
	m.instStatus["node2"] = instance.Status{Avail: peerAvail}
	m.instMonitor["node2"] = instance.Monitor{
		State:               instance.MonitorStateIdle,
		OrchestrationID:     m.state.OrchestrationID,
		OrchestrationIsDone: true,
	}
	return m
}

// A live move brings the container to the destination running, with no
// start: the object is up there when the source is done, which is the
// placement reached, not a start that failed.
func TestAPlacementAMoveHandedOverIsReached(t *testing.T) {
	m := newPlacedAtTestManager(status.Up, status.Up, status.Down)
	m.orchestrateFailoverPlacedStartFromStopped()
	require.Equal(t, instance.MonitorStateStartSuccess, m.state.State)
	require.False(t, m.state.OrchestrationIsDone, "the started state ends it, as for any start")
}

// The source still holding the object up once it is done is a stop that did
// not happen, and the placement ends on it.
func TestAPlacementTheSourceDidNotHandOverEnds(t *testing.T) {
	m := newPlacedAtTestManager(status.Up, status.Down, status.Up)
	m.orchestrateFailoverPlacedStartFromStopped()
	require.Equal(t, instance.MonitorStateIdle, m.state.State)
	require.True(t, m.state.OrchestrationIsDone)
}

// A system event is not worth a status refresh while an action of the object
// runs, here or on a peer: the action ends on a status of its own, and the
// event is likely its doing, read half way.
func TestNoEventRefreshWhileAnActionRuns(t *testing.T) {
	m := newTestManager(&pubSpy{})
	m.instMonitor["node2"] = instance.Monitor{State: instance.MonitorStateIdle}
	require.True(t, m.canRefreshOnEvent())

	m.instMonitor["node2"] = instance.Monitor{State: instance.MonitorStateStopProgress}
	require.False(t, m.canRefreshOnEvent(), "a peer stopping, as the source of a live move")

	m.instMonitor["node2"] = instance.Monitor{State: instance.MonitorStateIdle}
	m.state.State = instance.MonitorStateStartProgress
	require.False(t, m.canRefreshOnEvent(), "the local instance starting")
}
