package nmon

import "github.com/opensvc/om3/v3/core/node"

func (t *Manager) orchestrateAborted() {
	t.log.Infof("abort orchestration: unset global expect")
	t.change = true
	t.state.GlobalExpect = node.MonitorGlobalExpectNone
	t.endOrchestration()

	// The success state of an action done is left for idle: the node
	// waits in it for its orchestration to end, which the abort does.
	switch t.state.State {
	case node.MonitorStateFreezeSuccess, node.MonitorStateUnfreezeSuccess:
		t.state.State = node.MonitorStateIdle
	}

	// drained is abortable
	switch t.state.LocalExpect {
	case node.MonitorLocalExpectDrained:
		t.state.LocalExpect = node.MonitorLocalExpectNone
	}

	t.updateIfChange()
}
