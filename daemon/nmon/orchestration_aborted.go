package nmon

import "github.com/opensvc/om3/v3/core/node"

func (t *Manager) orchestrateAborted() {
	t.log.Infof("abort orchestration: unset global expect")
	t.change = true
	t.state.GlobalExpect = node.MonitorGlobalExpectNone

	// drained is abortable
	switch t.state.LocalExpect {
	case node.MonitorLocalExpectDrained:
		t.state.LocalExpect = node.MonitorLocalExpectNone
	}

	t.updateIfChange()
}
