package imon

import (
	"testing"

	"github.com/stretchr/testify/require"

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
func TestStoppedFlagLoweredBy(t *testing.T) {
	for _, tc := range []struct {
		name            string
		localAvail      status.T
		isObjectStarted bool
		wantLowered     bool
	}{
		{"the instance is up", status.Up, false, true},
		{"the instance is standby up", status.StandbyUp, false, true},
		{"the instance is down, the object runs all it should", status.Down, true, true},
		{"the instance is down, the object is short of what it should run", status.Down, false, false},
		{"the instance is standby down, the object is short of what it should run", status.StandbyDown, false, false},
		{"the instance is warn, the object is short of what it should run", status.Warn, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reason := stoppedFlagLoweredBy(tc.localAvail, tc.isObjectStarted)
			require.Equal(t, tc.wantLowered, reason != "", "reason %q", reason)
		})
	}
}
