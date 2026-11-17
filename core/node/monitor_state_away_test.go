package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A node stopped, starting, rejoining or shutting down is away: the
// orchestrations of its peers do not wait for it.
func TestMonitorStateIsAway(t *testing.T) {
	for _, s := range []MonitorState{MonitorStateInit, MonitorStateRejoin, MonitorStateMaintenance, MonitorStateShutdownProgress, MonitorStateShutdownSuccess, MonitorStateShutdownFailure} {
		assert.Truef(t, s.IsAway(), "%s", s)
	}
	for _, s := range []MonitorState{MonitorStateIdle, MonitorStateFreezeProgress, MonitorStateDrainProgress, MonitorStateUpgrade} {
		assert.Falsef(t, s.IsAway(), "%s", s)
	}
}

// A node upgrading its packages runs no object orchestration, its daemon
// restarting, while the upgrade, a node orchestration, does not count it
// away.
func TestMonitorStateIsAwayFromObjects(t *testing.T) {
	assert.True(t, MonitorStateUpgrade.IsAwayFromObjects())
	assert.False(t, MonitorStateUpgrade.IsAway())
	assert.True(t, MonitorStateRejoin.IsAwayFromObjects())
	assert.False(t, MonitorStateIdle.IsAwayFromObjects())
}
