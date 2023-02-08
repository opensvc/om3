package imon

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/topology"
)

func (o *imon) orchestrateUnset() {
	o.clearStartFailed()
	if o.objStatus.Orchestrate == "ha" {
		o.orchestrateHAStart()
		o.orchestrateHAStop()
	}
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
	switch o.state.State {
	case instance.MonitorStateReady:
		o.cancelReadyState()
	}
	if v, _ := o.isStartable(); !v {
		return
	}
	o.orchestrateStarted()
}

func (o *imon) clearStartFailed() {
	if o.state.State != instance.MonitorStateStartFailed {
		return
	}
	if o.objStatus.Avail != status.Up {
		return
	}
	for _, instanceMonitor := range o.instMonitor {
		switch instanceMonitor.GlobalExpect {
		case instance.MonitorGlobalExpectEmpty:
		case instance.MonitorGlobalExpectUnset:
		default:
			return
		}
	}
	o.log.Info().Msgf("clear instance start failed: the object is up")
	o.transitionTo(instance.MonitorStateIdle)
}
