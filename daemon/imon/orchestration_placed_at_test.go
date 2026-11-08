package imon

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resource"
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

// A placement whose stop failed with the instance up, all of it, as a live
// move refused and rolled back, leaves the instance as it found it: idle, its
// monitoring and its restarts on.
func TestAPlacementStopFailedWithTheInstanceUpGoesBackToIdle(t *testing.T) {
	m := newPlacedAtTestManager(status.Up, status.Up, status.Down)
	m.state.State = instance.MonitorStateStopFailure
	m.state.LocalExpect = instance.MonitorLocalExpectNone
	m.orchestratePlacedStopFromStopFailure()
	require.Equal(t, instance.MonitorStateStopFailure, m.state.State, "failed until the orchestration ends, for it to say so")
	require.True(t, m.state.OrchestrationIsDone)

	m.endOrchestration()
	require.Equal(t, instance.MonitorStateIdle, m.state.State)
	require.Equal(t, instance.MonitorLocalExpectStarted, m.state.LocalExpect, "the monitoring back on")
}

// A stop failed half way, the instance partly up, is left to the operator,
// the monitoring off.
func TestAPlacementStopFailedHalfWayKeepsTheMonitoringOff(t *testing.T) {
	m := newPlacedAtTestManager(status.Warn, status.Warn, status.Down)
	m.state.State = instance.MonitorStateStopFailure
	m.state.LocalExpect = instance.MonitorLocalExpectNone
	m.orchestratePlacedStopFromStopFailure()
	require.Equal(t, instance.MonitorStateStopFailure, m.state.State)
	require.Equal(t, instance.MonitorLocalExpectNone, m.state.LocalExpect)
	require.True(t, m.state.OrchestrationIsDone)
}

// newWaitFilesTestManager returns the manager of the destination of a
// switch, node1, whose source node2 stopped listing the definition of a kvm
// domain the destination has not fetched: the definition the object was
// just given, which the status of the source listed first at the end of its
// stop. It records the actions queued.
func newWaitFilesTestManager(t *testing.T) (*Manager, string, *[]instance.MonitorState) {
	m := newPlacedAtTestManager(status.Down, status.Down, status.Up)
	m.state.GlobalExpectOptions = instance.MonitorGlobalExpectOptionsPlacedAt{Destination: []string{"node1"}}
	m.recordFilesSource("node2")
	name := filepath.Join(t.TempDir(), "vm.xml")
	m.instStatus["node2"] = instance.Status{Avail: status.Down, Resources: instance.ResourceStatuses{
		"container#1": {Status: status.Down, Files: resource.Files{{Name: name, Checksum: "new"}}},
	}}
	m.instStatus[m.localhost] = instance.Status{Avail: status.Down, Resources: instance.ResourceStatuses{
		"container#1": {Status: status.Down},
	}}
	queued := make([]instance.MonitorState, 0)
	testQueueAction = func(newState instance.MonitorState) {
		queued = append(queued, newState)
	}
	t.Cleanup(func() { testQueueAction = nil })
	return m, name, &queued
}

// A destination with files pending does not start, and starts once fetched:
// it started without the definition the source listed, and failed on
// "definition not found".
func TestADestinationWithFilesPendingDoesNotStartStartsOnceFetched(t *testing.T) {
	m, name, queued := newWaitFilesTestManager(t)
	m.orchestrateFailoverPlacedStart()
	require.Equal(t, instance.MonitorStateWaitFiles, m.state.State)
	require.Empty(t, *queued, "not started")

	m.orchestrateFailoverPlacedStart()
	require.Equal(t, instance.MonitorStateWaitFiles, m.state.State, "still waiting")
	require.Empty(t, *queued)

	m.onFetchDone(cmdFetchDone{Files: ridFiles{{rid: "container#1", File: &resource.File{Name: name, Checksum: "new"}}}})
	m.orchestrateFailoverPlacedStart()
	require.Equal(t, []instance.MonitorState{instance.MonitorStateStartProgress}, *queued, "started once fetched")
	require.True(t, m.waitFilesSince.IsZero())
}

// A file the destination holds already in the version the source lists is
// not waited for.
func TestADestinationHoldingTheFilesStartsRightAway(t *testing.T) {
	m, name, queued := newWaitFilesTestManager(t)
	m.files.states[name] = ridFile{rid: "container#1", File: &resource.File{Name: name, Checksum: "new"}}
	m.orchestrateFailoverPlacedStart()
	require.Equal(t, []instance.MonitorState{instance.MonitorStateStartProgress}, *queued)
}

// The files not fetched after the timeout fail the placement, naming them
// and the source, the instance not started.
func TestADestinationWaitingForFilesTimesOut(t *testing.T) {
	m, name, queued := newWaitFilesTestManager(t)
	m.orchestrateFailoverPlacedStart()
	require.Equal(t, instance.MonitorStateWaitFiles, m.state.State)

	sources, files := m.pendingSourceFiles()
	require.Equal(t, []string{"node2"}, sources, "the source is named")
	require.Equal(t, []string{name}, files, "the file is named")

	m.waitFilesSince = time.Now().Add(-2 * waitFilesTimeout)
	m.orchestrateFailoverPlacedStart()
	require.Empty(t, *queued, "not started anyway")
	require.Equal(t, instance.MonitorStateStartFailure, m.state.State)
	require.True(t, m.state.State.IsOneOf(instance.MonitorStatesFailure...), "a failed state says the switch failed")
	require.True(t, m.state.OrchestrationIsDone)
}

// An abort leaves the wait, the instance back to idle.
func TestAnAbortLeavesTheWaitForFiles(t *testing.T) {
	m, _, _ := newWaitFilesTestManager(t)
	m.orchestrateFailoverPlacedStart()
	require.Equal(t, instance.MonitorStateWaitFiles, m.state.State)
	m.orchestrateAborted()
	require.Equal(t, instance.MonitorStateIdle, m.state.State)
}
