package nmon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type pubSink struct{}

func (pubSink) Pub(pubsub.Messager, ...pubsub.Label) {}

func newFreezeTestManager(t *testing.T, state node.MonitorState, globalExpect node.MonitorGlobalExpect) *Manager {
	t.Helper()
	t.Cleanup(func() { node.MonitorData.Unset("node1") })
	return &Manager{
		localhost:   "node1",
		log:         plog.NewDefaultLogger(),
		publisher:   pubSink{},
		state:       node.Monitor{State: state, GlobalExpect: globalExpect},
		nodeMonitor: make(map[string]node.Monitor),
	}
}

// A thaw adopted while the node is still in the frozen state its freeze left
// it in, its orchestration ended elsewhere first, goes on from idle: the node
// stayed frozen for good, an abort not helping either.
func TestAThawFromTheFrozenStateGoesOn(t *testing.T) {
	m := newFreezeTestManager(t, node.MonitorStateFreezeSuccess, node.MonitorGlobalExpectUnfrozen)
	// Already thawed: the thaw is reached at once, no action run.
	m.orchestrateUnfrozen()
	assert.Equal(t, node.MonitorStateIdle, m.state.State)
	assert.Equal(t, node.MonitorGlobalExpectNone, m.state.GlobalExpect)

	m = newFreezeTestManager(t, node.MonitorStateUnfreezeSuccess, node.MonitorGlobalExpectFrozen)
	m.nodeStatus.FrozenAt = time.Now()
	m.orchestrateFrozen()
	assert.Equal(t, node.MonitorStateIdle, m.state.State)
	assert.Equal(t, node.MonitorGlobalExpectNone, m.state.GlobalExpect)
}

// An abort leaves the success state of an action done for idle.
func TestAnAbortLeavesTheFrozenState(t *testing.T) {
	m := newFreezeTestManager(t, node.MonitorStateFreezeSuccess, node.MonitorGlobalExpectAborted)
	m.orchestrateAborted()
	assert.Equal(t, node.MonitorStateIdle, m.state.State)
}
