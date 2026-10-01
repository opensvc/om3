package imon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
)

// A failover object runs all it should when it is up. A flex object is up
// from its first instance on, and runs all it should when it has as many up
// instances as its target.
func TestIsObjectStarted(t *testing.T) {
	for _, tc := range []struct {
		name   string
		topo   topology.T
		avail  status.T
		up     int
		target int
		want   bool
	}{
		{"failover, up", topology.Failover, status.Up, 1, 0, true},
		{"failover, down", topology.Failover, status.Down, 0, 0, false},
		{"failover, warn", topology.Failover, status.Warn, 0, 0, false},
		{"flex, short of its target", topology.Flex, status.Up, 1, 2, false},
		{"flex, at its target", topology.Flex, status.Up, 2, 2, true},
		{"flex, over its target", topology.Flex, status.Up, 3, 2, true},
		{"flex, down", topology.Flex, status.Down, 0, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isObjectStarted(tc.topo, tc.avail, tc.up, tc.target))
		})
	}
}

// The stopped flag of an instance is lowered when the instance is up, or
// when the object runs all it should. An instance stopped alone in a flex
// object left short of its target stays flagged: the object is up, and the
// next candidate is the one to start.
//
// Nothing lowers it on a status that can not tell what the stop left: while
// an action runs on the instance, or on a status not updated since the flag
// was raised. A stop raises it before its resources are down, and an
// instance seen up then was lowered, and started back by the daemon once the
// stop was over (#1142).
func TestStoppedFlagLoweredBy(t *testing.T) {
	stoppedAt := time.Date(2026, 10, 1, 15, 36, 5, 0, time.UTC)
	after := stoppedAt.Add(time.Second)
	for _, tc := range []struct {
		name            string
		state           instance.MonitorState
		localAvail      status.T
		updatedAt       time.Time
		isObjectStarted bool
		wantLowered     bool
	}{
		{"the instance is up", instance.MonitorStateIdle, status.Up, after, false, true},
		{"the instance is standby up", instance.MonitorStateIdle, status.StandbyUp, after, false, true},
		{"the instance is down, the object runs all it should", instance.MonitorStateIdle, status.Down, after, true, true},
		{"the instance is down, the object is short of what it should run", instance.MonitorStateIdle, status.Down, after, false, false},
		{"the instance is standby down, the object is short of what it should run", instance.MonitorStateIdle, status.StandbyDown, after, false, false},
		{"the instance is warn, the object is short of what it should run", instance.MonitorStateIdle, status.Warn, after, false, false},
		{"the instance is up in the middle of its stop", instance.MonitorStateStopProgress, status.Up, after, false, false},
		{"the object runs all it should in the middle of a stop", instance.MonitorStateStopProgress, status.Down, after, true, false},
		{"the instance is up in the middle of its start", instance.MonitorStateStartProgress, status.Up, after, false, false},
		{"the instance is up once its stop failed", instance.MonitorStateStopFailure, status.Up, after, false, true},
		{"the update raising the flag, before the stop is known", instance.MonitorStateIdle, status.Up, stoppedAt, false, false},
		{"the update raising the flag, the object runs all it should", instance.MonitorStateIdle, status.Up, stoppedAt, true, false},
		{"a status evaluated before the flag was raised", instance.MonitorStateIdle, status.Up, stoppedAt.Add(-time.Second), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			local := instance.Status{Avail: tc.localAvail, UpdatedAt: tc.updatedAt, StoppedAt: stoppedAt}
			reason := stoppedFlagLoweredBy(tc.state, local, tc.isObjectStarted)
			require.Equal(t, tc.wantLowered, reason != "", "reason %q", reason)
		})
	}
}
