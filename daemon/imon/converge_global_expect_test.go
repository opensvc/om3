package imon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/util/plog"
)

// A peer running a later version may hold a global expect this agent has no
// name for. Adopting it, this node would never end its part of an
// orchestration it can not run, and the peers would wait for it for good, so
// it is left alone.
func TestConvergeGlobalExpectIgnoresAnUnknownOne(t *testing.T) {
	now := time.Now()
	m := &Manager{
		state: instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectNone, GlobalExpectUpdatedAt: now.Add(-time.Minute)},
		instMonitor: map[string]instance.Monitor{
			"n2": {GlobalExpect: instance.MonitorGlobalExpectUnknown, GlobalExpectUpdatedAt: now},
		},
	}
	m.convergeGlobalExpectFromRemote()
	assert.False(t, m.change)
	assert.Equal(t, instance.MonitorGlobalExpectNone, m.state.GlobalExpect)
}

// A peer whose instance a purge already deleted has no instance monitor any
// more, and never will: an abort does not wait for it, as the purge did not,
// while another orchestration still waits for the monitor of a peer node up.
func TestAnAbortDoesNotWaitForTheMonitorOfADeletedPeerInstance(t *testing.T) {
	now := time.Now()
	newManager := func(globalExpect instance.MonitorGlobalExpect) *Manager {
		return &Manager{
			localhost:   "n1",
			scopeNodes:  []string{"n1", "n2"},
			instMonitor: map[string]instance.Monitor{},
			nodeMonitor: map[string]node.Monitor{"n2": {}},
			instConfig:  instance.Config{UpdatedAt: now.Add(-time.Hour)},
			state:       instance.Monitor{GlobalExpect: globalExpect, GlobalExpectUpdatedAt: now},
			log:         plog.NewDefaultLogger(),
		}
	}
	assert.True(t, newManager(instance.MonitorGlobalExpectAborted).isConvergedGlobalExpect())
	assert.True(t, newManager(instance.MonitorGlobalExpectPurged).isConvergedGlobalExpect())
	assert.False(t, newManager(instance.MonitorGlobalExpectStarted).isConvergedGlobalExpect())
}
