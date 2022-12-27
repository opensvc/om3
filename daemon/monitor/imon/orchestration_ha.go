package imon

import (
	"opensvc.com/opensvc/core/topology"
)

func (o *imon) orchestrateHA() {
	if o.objStatus.Orchestrate != "ha" {
		return
	}
	o.orchestrateHAStart()
	o.orchestrateHAStop()
}

func (o *imon) orchestrateHAStop() {
	if o.objStatus.Topology != topology.Flex {
		return
	}
	if v, _ := o.isExtraInstance(); !v {
		return
	}
	o.stop()
}

func (o *imon) orchestrateHAStart() {
	if v, _ := o.isStartable(); !v {
		return
	}
	o.orchestrateStarted()
}
