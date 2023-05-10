package imon

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/topology"
)

func (o *imon) orchestrateNone() {
	o.clearStartFailed()
	o.clearBootFailed()
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
	// we are here because we are ha object with global expect None
	switch o.state.State {
	case instance.MonitorStateReady:
		o.cancelReadyState()
	case instance.MonitorStateStarted:
		// started means the action start has been done. This state is a
		// waiter step to verify if received started like local instance status
		// to transition state: started -> idle
		// It prevents unexpected transition state -> ready
		if o.isLocalStarted() {
			o.log.Info().Msg("local instance status is now started like, leave state started, set local expect started")
			o.state.LocalExpect = instance.MonitorLocalExpectStarted
			o.transitionTo(instance.MonitorStateIdle)
		}
		return
	}
	if v, _ := o.isStartable(); !v {
		return
	}
	if o.isLocalStarted() {
		return
	}
	o.orchestrateStarted()
}

// clearBootFailed clears the boot failed state when the following conditions are met:
//
// + object avail is UP
// + local avail is Down, StandbyDown, NotApplicable
// + global expect is none
func (o *imon) clearBootFailed() {
	if o.state.State != instance.MonitorStateBootFailed {
		return
	}
	if o.objStatus.Avail != status.Up {
		return
	}
	switch o.instStatus[o.localhost].Avail {
	case status.Down:
	case status.StandbyDown:
	case status.NotApplicable:
	default:
		return
	}
	for _, instanceMonitor := range o.instMonitor {
		switch instanceMonitor.GlobalExpect {
		case instance.MonitorGlobalExpectNone:
		default:
			return
		}
	}
	o.log.Info().Msgf("clear instance %s: local instance avail is %s, object avail is %s",
		o.state.State, o.instStatus[o.localhost].Avail, o.objStatus.Avail)
	o.transitionTo(instance.MonitorStateIdle)
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
		case instance.MonitorGlobalExpectNone:
		default:
			return
		}
	}
	o.log.Info().Msgf("clear instance start failed: the object is up")
	o.transitionTo(instance.MonitorStateIdle)
}
