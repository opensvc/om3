package imon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
)

// newWaitDestinationTestManager returns the manager of a switch source,
// node1 up, the destination node2 holding volume#1 with the blocker given.
func newWaitDestinationTestManager(blocker *resource.StartBlocker) *Manager {
	m := newTestManager(&pubSpy{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.ctx = ctx
	m.state.GlobalExpect = instance.MonitorGlobalExpectPlacedAt
	m.state.GlobalExpectOptions = instance.MonitorGlobalExpectOptionsPlacedAt{Destination: []string{"node2"}}
	m.instStatus["node1"] = instance.Status{Avail: status.Up, Resources: instance.ResourceStatuses{
		"volume#1": {Status: status.Up},
	}}
	m.instStatus["node2"] = instance.Status{Avail: status.Down, Resources: instance.ResourceStatuses{
		"volume#1": {Status: status.Down, StartBlocker: blocker},
	}}
	return m
}

// A destination a resync can bring up to date is waited for, a disk it does
// not have or a resource up to date nowhere else is not.
func TestUnliftableBlockers(t *testing.T) {
	m := newWaitDestinationTestManager(&resource.StartBlocker{Reason: "drbd r0 disk Outdated", Resync: true})
	require.Empty(t, m.unliftableBlockers([]string{"node2"}), "node1 holds volume#1 up: a resync lifts it")

	m.instStatus["node1"] = instance.Status{Avail: status.Down, Resources: instance.ResourceStatuses{
		"volume#1": {Status: status.Down},
	}}
	require.Equal(t, []string{"node2 volume#1: drbd r0 disk Outdated, and no other node holds it up to date to resync from"},
		m.unliftableBlockers([]string{"node2"}))

	m = newWaitDestinationTestManager(&resource.StartBlocker{Reason: "drbd r0 disk Diskless"})
	require.Equal(t, []string{"node2 volume#1: drbd r0 disk Diskless"}, m.unliftableBlockers([]string{"node2"}))
}

// The source of a switch waits, before it stops, for the destination to be
// ready to start, and goes on once it is.
func TestSetWaitDestination(t *testing.T) {
	m := newWaitDestinationTestManager(&resource.StartBlocker{Reason: "drbd r0 disk Outdated", Resync: true})
	require.True(t, m.setWaitDestination())
	require.Equal(t, instance.MonitorStateWaitDestination, m.state.State)

	m.instStatus["node2"] = instance.Status{Avail: status.Down, Resources: instance.ResourceStatuses{
		"volume#1": {Status: status.StandbyUp},
	}}
	require.False(t, m.setWaitDestination(), "ready: the stop goes on")
	require.Equal(t, instance.MonitorStateIdle, m.state.State)
}

// A destination not ready after the start timeout fails the switch, the
// source not stopped, back to idle when the orchestration ends.
func TestSetWaitDestinationTimesOut(t *testing.T) {
	m := newWaitDestinationTestManager(&resource.StartBlocker{Reason: "drbd r0 disk Inconsistent", Resync: true})
	require.True(t, m.setWaitDestination())
	m.waitDestinationSince = time.Now().Add(-2 * time.Hour)
	require.True(t, m.setWaitDestination())
	require.Equal(t, instance.MonitorStateWaitDestinationFailure, m.state.State)
	require.True(t, m.idleAfterOrchestration)
	require.True(t, m.state.State.IsOneOf(instance.MonitorStatesFailure...), "a failed state says the switch failed")
}

// A destination failing to prepare fails the switch, the source not stopped.
func TestSetWaitDestinationPrepareFailed(t *testing.T) {
	m := newWaitDestinationTestManager(&resource.StartBlocker{Reason: "drbd r0 disk Outdated", Resync: true})
	m.instMonitor["node2"] = instance.Monitor{State: instance.MonitorStatePrepareFailure}
	require.True(t, m.setWaitDestination())
	require.Equal(t, instance.MonitorStateWaitDestinationFailure, m.state.State)
}
