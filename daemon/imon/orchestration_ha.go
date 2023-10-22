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
			o.log.Info().Msgf("daemon: imon: %s: instance is now started, enable resource restart", o.path)
			o.state.LocalExpect = instance.MonitorLocalExpectStarted
			o.transitionTo(instance.MonitorStateIdle)
		}
		return
	}
	if v, reason := o.isStartable(); !v {
		if o.pendingCancel != nil && o.state.State == instance.MonitorStateReady {
			o.log.Info().Msgf("daemon: imon: %s: instance is not startable, clear the ready state: %s", o.path, reason)
			o.clearPending()
			o.transitionTo(instance.MonitorStateIdle)
		}
		return
	}
	if o.isLocalStarted() {
		return
	}
	o.orchestrateStarted()
}

// clearBootFailed clears the boot failed state when the following conditions are met:
//
// + local avail is Down, StandbyDown, NotApplicable
// + global expect is none
func (o *imon) clearBootFailed() {
	if o.state.State != instance.MonitorStateBootFailed {
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
	o.log.Info().Msgf("daemon: imon: %s: clear instance %s: local instance avail is %s, object avail is %s",
		o.path, o.state.State, o.instStatus[o.localhost].Avail, o.objStatus.Avail)
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
	o.log.Info().Msgf("daemon: imon: %s: clear instance start failed: the object is up", o.path)
	o.transitionTo(instance.MonitorStateIdle)
}
