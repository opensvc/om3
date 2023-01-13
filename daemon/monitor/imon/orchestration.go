package imon

import (
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/instance"
)

// orchestrate from omon vs global expect
func (o *imon) orchestrate() {
	if _, ok := o.instStatus[o.localhost]; !ok {
		return
	}
	if _, ok := o.nodeStatus[o.localhost]; !ok {
		return
	}
	if !o.isConvergedGlobalExpect() {
		return
	}

	switch o.state.GlobalExpect {
	case instance.MonitorGlobalExpectAborted:
		o.orchestrateAborted()
	}

	if nodeMonitor, ok := o.nodeMonitor[o.localhost]; !ok {
		return
	} else if nodeMonitor.State != cluster.NodeMonitorStateIdle {
		return
	}

	o.orchestrateResourceRestart()

	switch o.state.GlobalExpect {
	case instance.MonitorGlobalExpectUnset:
		o.orchestrateUnset()
	case instance.MonitorGlobalExpectFrozen:
		o.orchestrateFrozen()
	case instance.MonitorGlobalExpectProvisioned:
		o.orchestrateProvisioned()
	case instance.MonitorGlobalExpectPlaced:
		o.orchestratePlaced()
	case instance.MonitorGlobalExpectPlacedAt:
		o.orchestratePlacedAt()
	case instance.MonitorGlobalExpectPurged:
		o.orchestratePurged()
	case instance.MonitorGlobalExpectStarted:
		o.orchestrateStarted()
	case instance.MonitorGlobalExpectStopped:
		o.orchestrateStopped()
	case instance.MonitorGlobalExpectThawed:
		o.orchestrateThawed()
	case instance.MonitorGlobalExpectUnprovisioned:
		o.orchestrateUnprovisioned()
	}
	o.updateIfChange()
}
