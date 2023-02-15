package nmon

import "github.com/opensvc/om3/core/node"

func (o *nmon) orchestrate() {
	switch o.state.State {
	case node.MonitorStateZero:
		return
	case node.MonitorStateRejoin:
		return
	}

	if !o.isConvergedGlobalExpect() {
		return
	}

	switch o.state.LocalExpect {
	case node.MonitorLocalExpectZero:
	case node.MonitorLocalExpectNone:
	case node.MonitorLocalExpectDrained:
		o.orchestrateDrained()
	}

	switch o.state.GlobalExpect {
	case node.MonitorGlobalExpectZero:
	case node.MonitorGlobalExpectNone:
	case node.MonitorGlobalExpectAborted:
		o.orchestrateAborted()
	case node.MonitorGlobalExpectFrozen:
		o.orchestrateFrozen()
	case node.MonitorGlobalExpectThawed:
		o.orchestrateThawed()
	}
	o.updateIfChange()
}
