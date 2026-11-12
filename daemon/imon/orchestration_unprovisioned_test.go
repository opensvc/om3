package imon

import (
	"context"
	"testing"
	"time"

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

// An unprovision that succeeded with the instance still read provisioned
// waits for a status taken after it, asking a refresh once. A status taken
// after it that still reads the instance provisioned, as a resource flagged
// provisioned the unprovision removed, ends the orchestration as an
// unprovision failure, rather than wait for ever.
func TestAnUnprovisionSucceededLeavingTheInstanceProvisionedFails(t *testing.T) {
	m := newUnprovisionedTestManager(instance.MonitorStateUnprovisionSuccess)
	endedAt := time.Now()
	m.state.StateUpdatedAt = endedAt
	m.instStatus[m.localhost] = instance.Status{
		Avail:       status.Down,
		Provisioned: provisioned.Mixed,
		UpdatedAt:   endedAt.Add(-time.Second),
	}

	m.orchestrateUnprovisioned()
	require.Equal(t, instance.MonitorStateUnprovisionSuccess, m.state.State, "a status taken before the end decides nothing")
	require.False(t, m.state.OrchestrationIsDone)
	require.True(t, m.unprovisionStatusAskedFor.Equal(endedAt), "a status refresh asked")

	m.instStatus[m.localhost] = instance.Status{
		Avail:       status.Down,
		Provisioned: provisioned.Mixed,
		UpdatedAt:   endedAt.Add(time.Second),
	}
	m.orchestrateUnprovisioned()
	require.Equal(t, instance.MonitorStateUnprovisionFailure, m.state.State)
	require.True(t, m.state.OrchestrationIsDone, "the orchestration ends")
}

// An unprovision that succeeded and left the instance unprovisioned ends the
// orchestration as before.
func TestAnUnprovisionSucceededLeavingTheInstanceUnprovisionedEnds(t *testing.T) {
	m := newUnprovisionedTestManager(instance.MonitorStateUnprovisionSuccess)
	m.state.StateUpdatedAt = time.Now()
	m.instStatus[m.localhost] = instance.Status{
		Avail:       status.Down,
		Provisioned: provisioned.False,
		UpdatedAt:   time.Now().Add(time.Second),
	}
	m.orchestrateUnprovisioned()
	require.True(t, m.state.OrchestrationIsDone)
	require.Equal(t, instance.MonitorStateIdle, m.state.State)
}
