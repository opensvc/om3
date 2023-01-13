package imon

import (
	"time"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/status"
)

var (
	stopDuration = 10 * time.Second
)

func (o *imon) orchestrateStopped() {
	o.freezeStop()
}

func (o *imon) freezeStop() {
	switch o.state.State {
	case instance.MonitorStateIdle:
		o.doFreezeStop()
	case instance.MonitorStateFrozen:
		o.doStop()
	case instance.MonitorStateReady:
		o.stoppedFromReady()
	case instance.MonitorStateFreezing:
		// wait for the freeze exec to end
	case instance.MonitorStateStopping:
		// avoid multiple concurrent stop execs
	case instance.MonitorStateStopFailed:
		// avoid a retry-loop
	case instance.MonitorStateStartFailed:
		o.stoppedFromFailed()
	default:
		o.log.Error().Msgf("don't know how to freeze and stop from %s", o.state.State)
	}
}

// stop stops the object but does not freeze.
// This func must be called by orchestrations that know the ha auto-start will
// not starts it back (ex: auto-stop), or that want the restart (ex: restart).
func (o *imon) stop() {
	switch o.state.State {
	case instance.MonitorStateIdle:
		o.doStop()
	case instance.MonitorStateReady:
		o.stoppedFromReady()
	case instance.MonitorStateFrozen:
		// honor the frozen state
	case instance.MonitorStateFreezing:
		// wait for the freeze exec to end
	case instance.MonitorStateStopping:
		// avoid multiple concurrent stop execs
	case instance.MonitorStateStopFailed:
		// avoid a retry-loop
	case instance.MonitorStateStartFailed:
		o.stoppedFromFailed()
	default:
		o.log.Error().Msgf("don't know how to stop from %s", o.state.State)
	}
}

func (o *imon) stoppedFromThawed() {
	o.doTransitionAction(o.freeze, instance.MonitorStateFreezing, instance.MonitorStateIdle, instance.MonitorStateFreezeFailed)
}

// doFreeze handle global expect stopped orchestration from idle
//
// local thawed => freezing to reach frozen
// else         => stopping
func (o *imon) doFreezeStop() {
	if o.instStatus[o.localhost].IsThawed() {
		o.doTransitionAction(o.freeze, instance.MonitorStateFreezing, instance.MonitorStateFrozen, instance.MonitorStateFreezeFailed)
		return
	} else {
		o.doStop()
	}
}

func (o *imon) doFreeze() {
	if o.instStatus[o.localhost].IsThawed() {
		o.doTransitionAction(o.freeze, instance.MonitorStateFreezing, instance.MonitorStateFrozen, instance.MonitorStateFreezeFailed)
		return
	}
}

func (o *imon) doStop() {
	if o.stoppedClearIfReached() {
		return
	}
	o.createPendingWithDuration(stopDuration)
	o.doAction(o.crmStop, instance.MonitorStateStopping, instance.MonitorStateIdle, instance.MonitorStateStopFailed)
}

func (o *imon) stoppedFromReady() {
	o.log.Info().Msg("reset ready state global expect is stopped")
	o.clearPending()
	o.change = true
	o.state.State = instance.MonitorStateIdle
	o.stoppedClearIfReached()
}

func (o *imon) stoppedFromFailed() {
	o.log.Info().Msg("reset %s state global expect is stopped")
	o.change = true
	o.state.State = instance.MonitorStateIdle
	o.stoppedClearIfReached()
}

func (o *imon) stoppedFromAny() {
	if o.pendingCancel == nil {
		o.stoppedClearIfReached()
		return
	}
}

func (o *imon) stoppedClearIfReached() bool {
	if o.isLocalStopped() {
		o.loggerWithState().Info().Msg("instance state is stopped, unset global expect")
		o.change = true
		o.state.GlobalExpect = instance.MonitorGlobalExpectUnset
		o.state.LocalExpect = instance.MonitorLocalExpectUnset
		o.state.State = instance.MonitorStateIdle
		o.clearPending()
		return true
	}
	return false
}

func (o *imon) isLocalStopped() bool {
	instStatus := o.instStatus[o.localhost]
	switch instStatus.Avail {
	case status.NotApplicable, status.Undef:
		return true
	case status.Down:
		return true
	case status.StandbyDown:
		return true
	default:
		return false
	}
}
