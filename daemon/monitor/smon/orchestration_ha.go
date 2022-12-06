package smon

import "opensvc.com/opensvc/core/topology"

func (o *smon) orchestrateHA() {
	nodeStatus := o.nodeStatus[o.localhost]
	if !nodeStatus.Frozen.IsZero() {
		return
	}
	instStatus := o.instStatus[o.localhost]
	if instStatus.Orchestrate != "ha" {
		return
	}
	o.orchestrateHAStart()
	o.orchestrateHAStop()
}

func (o *smon) orchestrateHAStop() {
	instStatus := o.instStatus[o.localhost]
	if instStatus.Topology != topology.Flex {
		return
	}
	if v, _ := o.isExtraInstance(); !v {
		return
	}
	o.stop()
}

func (o *smon) orchestrateHAStart() {
	if v, _ := o.isStartable(); !v {
		return
	}
	o.orchestrateStarted()
}
