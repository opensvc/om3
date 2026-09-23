package imon

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
)

// newProvisionedTestManager returns a manager with a provisioned
// orchestration running on it, and a cancelled context: the decisions are what
// is under test here, not the publishing the daemon tests cover.
func newProvisionedTestManager(state instance.MonitorState) *Manager {
	m := newTestManager(&pubSpy{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.state.State = state
	m.state.GlobalExpect = instance.MonitorGlobalExpectProvisioned
	m.state.OrchestrationID = uuid.New()

	// An object whose resources have nothing to provision, down because the
	// start the orchestration queued did not work.
	m.instStatus[m.localhost] = instance.Status{
		Avail:       status.Down,
		Provisioned: provisioned.NotApplicable,
	}
	return m
}

// A provisioned orchestration provisions what needs provisioning and starts
// what does not, so the start it queues is its own work and a failed one ends
// it.
//
// This used to end only on a provision failure. An object whose every
// resource reports its provisioned state as n/a is never provisioned, only
// started, so a start that failed left the orchestration running with nothing
// left to run it: the instance monitor does not retry a failed action, and
// `om <path> provision --wait` waited out its whole timeout on an object that
// had already given up.
func TestAProvisionedOrchestrationEndsOnAStartFailure(t *testing.T) {
	m := newProvisionedTestManager(instance.MonitorStateStartFailure)

	require.True(t, m.provisionedClearIfReached(), "the orchestration must end")
	require.True(t, m.state.OrchestrationIsDone, "the orchestration must be marked done")
	require.Equal(t, instance.MonitorStateStartFailure, m.state.State,
		"the failure must linger, so an operator can see what failed")
}

// The same, on the node of an object whose peers are still idle: the local
// instance gave up, and says so, whatever the others end up doing.
func TestAProvisionedOrchestrationEndsOnAStartFailureWithIdlePeers(t *testing.T) {
	m := newProvisionedTestManager(instance.MonitorStateStartFailure)
	m.instMonitor["node2"] = instance.Monitor{State: instance.MonitorStateIdle}
	m.instStatus["node2"] = instance.Status{
		Avail:       status.Down,
		Provisioned: provisioned.NotApplicable,
	}

	require.True(t, m.provisionedClearIfReached(), "the orchestration must end")
	require.True(t, m.state.OrchestrationIsDone, "the orchestration must be marked done")
}

// A provision failure ends it as it always did.
func TestAProvisionedOrchestrationEndsOnAProvisionFailure(t *testing.T) {
	m := newProvisionedTestManager(instance.MonitorStateProvisionFailure)

	require.True(t, m.provisionedClearIfReached(), "the orchestration must end")
	require.True(t, m.state.OrchestrationIsDone, "the orchestration must be marked done")
}

// Nothing ends it while the instance is merely down and idle: that is the
// state the orchestration is there to act on, by queueing the start.
func TestAProvisionedOrchestrationRunsOnWhileTheInstanceIsIdle(t *testing.T) {
	m := newProvisionedTestManager(instance.MonitorStateIdle)

	require.False(t, m.provisionedClearIfReached(), "the orchestration must run on")
	require.False(t, m.state.OrchestrationIsDone, "the orchestration must not be marked done")
}
