package nmon

import "github.com/opensvc/om3/v3/core/node"

func (t *Manager) orchestrate() {
	switch t.state.State {
	case node.MonitorStateInit:
		return
	case node.MonitorStateRejoin:
		return
	}

	if !t.isConvergedGlobalExpect() {
		return
	}

	switch t.state.LocalExpect {
	case node.MonitorLocalExpectInit:
	case node.MonitorLocalExpectNone:
	case node.MonitorLocalExpectDrained:
		t.orchestrateDrained()
	}

	switch t.state.GlobalExpect {
	case node.MonitorGlobalExpectInit:
	case node.MonitorGlobalExpectNone:
	case node.MonitorGlobalExpectAborted:
		t.orchestrateAborted()
	case node.MonitorGlobalExpectFrozen:
		t.orchestrateFrozen()
	case node.MonitorGlobalExpectUnfrozen:
		t.orchestrateUnfrozen()
	}
	t.updateIfChange()
}
