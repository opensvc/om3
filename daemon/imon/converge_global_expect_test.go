package imon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
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
