package nmon

import "opensvc.com/opensvc/core/cluster"

func (o *nmon) orchestrate() {
	switch o.state.State {
	case cluster.NodeMonitorStateInit:
		return
	case cluster.NodeMonitorStateRejoin:
		return
	}

	if !o.isConvergedGlobalExpect() {
		return
	}

	switch o.state.LocalExpect {
	case cluster.NodeMonitorLocalExpectUnset:
	case cluster.NodeMonitorLocalExpectDrained:
		o.orchestrateDrained()
	}

	switch o.state.GlobalExpect {
	case cluster.NodeMonitorGlobalExpectUnset:
	case cluster.NodeMonitorGlobalExpectAborted:
		o.orchestrateAborted()
	case cluster.NodeMonitorGlobalExpectFrozen:
		o.orchestrateFrozen()
	case cluster.NodeMonitorGlobalExpectThawed:
		o.orchestrateThawed()
	}
	o.updateIfChange()
}
