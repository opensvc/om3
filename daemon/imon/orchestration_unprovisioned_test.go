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

// newUnprovisionedTestManager returns a manager with an unprovisioned
// orchestration running on it, its local instance still provisioned, and a
// cancelled context.
func newUnprovisionedTestManager(state instance.MonitorState) *Manager {
	m := newTestManager(&pubSpy{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.state.State = state
	m.state.GlobalExpect = instance.MonitorGlobalExpectUnprovisioned
	m.state.OrchestrationID = uuid.New()
	m.instStatus[m.localhost] = instance.Status{
		Avail:       status.Warn,
		Provisioned: provisioned.Mixed,
	}
	return m
}

// An unprovision that failed ends the orchestration on its node, whatever
// the peers did: it is not retried, so the peers done with theirs would
// otherwise wait for it forever.
func TestAnUnprovisionedOrchestrationEndsOnAnUnprovisionFailure(t *testing.T) {
	m := newUnprovisionedTestManager(instance.MonitorStateUnprovisionFailure)
	m.instMonitor["node2"] = instance.Monitor{State: instance.MonitorStateIdle, OrchestrationIsDone: true}
	m.instStatus["node2"] = instance.Status{
		Avail:       status.Down,
		Provisioned: provisioned.False,
	}

	m.orchestrateUnprovisioned()
	require.True(t, m.state.OrchestrationIsDone, "the orchestration must be marked done")
	require.Equal(t, instance.MonitorStateUnprovisionFailure, m.state.State,
		"the failure must linger, so an operator can see what failed")
}

// An unprovision that failed and left the instance unprovisioned anyway
// reached the state asked for, and ends the orchestration as a success.
func TestAnUnprovisionFailureThatReachedTheStateEndsIdle(t *testing.T) {
	m := newUnprovisionedTestManager(instance.MonitorStateUnprovisionFailure)
	m.instStatus[m.localhost] = instance.Status{
		Avail:       status.Down,
		Provisioned: provisioned.False,
	}

	m.orchestrateUnprovisioned()
	require.True(t, m.state.OrchestrationIsDone, "the orchestration must be marked done")
	require.Equal(t, instance.MonitorStateIdle, m.state.State, "the state reached is no failure")
}

// A succeeded unprovision still waits for its instance to report it is not
// provisioned any more.
func TestAnUnprovisionSuccessWaitsForTheStatus(t *testing.T) {
	m := newUnprovisionedTestManager(instance.MonitorStateUnprovisionSuccess)

	m.orchestrateUnprovisioned()
	require.False(t, m.state.OrchestrationIsDone, "the orchestration must not be marked done")
}
