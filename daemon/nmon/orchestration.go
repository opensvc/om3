package nmon

import "github.com/opensvc/om3/core/node"

func (t *Manager) orchestrate() {
	switch t.state.State {
	case node.MonitorStateZero:
		return
	case node.MonitorStateRejoin:
		return
	}

	if !t.isConvergedGlobalExpect() {
		return
	}

	switch t.state.LocalExpect {
	case node.MonitorLocalExpectZero:
	case node.MonitorLocalExpectNone:
	case node.MonitorLocalExpectDrained:
		t.orchestrateDrained()
	}

	switch t.state.GlobalExpect {
	case node.MonitorGlobalExpectZero:
	case node.MonitorGlobalExpectNone:
	case node.MonitorGlobalExpectAborted:
		t.orchestrateAborted()
	case node.MonitorGlobalExpectFrozen:
		t.orchestrateFrozen()
	case node.MonitorGlobalExpectThawed:
		t.orchestrateThawed()
	}
	t.updateIfChange()
}
