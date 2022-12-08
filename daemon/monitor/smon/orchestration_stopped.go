package smon

import (
	"time"

	"opensvc.com/opensvc/core/status"
)

var (
	stopDuration = 10 * time.Second
)

func (o *smon) orchestrateStopped() {
	o.freezeStop()
}

func (o *smon) freezeStop() {
	switch o.state.Status {
	case statusIdle:
		o.doFreezeStop()
	case statusFrozen:
		o.doStop()
	case statusReady:
		o.stoppedFromReady()
	case statusFreezing:
		// wait for the freeze exec to end
	case statusStopping:
		// avoid multiple concurrent stop execs
	case statusStopFailed:
		// avoid a retry-loop
	case statusStartFailed:
		o.stoppedFromFailed()
	default:
		o.log.Error().Msgf("don't know how to freeze and stop from %s", o.state.Status)
	}
}

// stop stops the object but does not freeze.
// This func must be called by orchestrations that know the ha auto-start will
// not starts it back (ex: auto-stop), or that want the restart (ex: restart).
func (o *smon) stop() {
	switch o.state.Status {
	case statusIdle:
		o.doStop()
	case statusReady:
		o.stoppedFromReady()
	case statusFrozen:
		// honor the frozen state
	case statusFreezing:
		// wait for the freeze exec to end
	case statusStopping:
		// avoid multiple concurrent stop execs
	case statusStopFailed:
		// avoid a retry-loop
	case statusStartFailed:
		o.stoppedFromFailed()
	default:
		o.log.Error().Msgf("don't know how to stop from %s", o.state.Status)
	}
}

func (o *smon) stoppedFromThawed() {
	o.doTransitionAction(o.freeze, statusFreezing, statusIdle, statusFreezeFailed)
}

// doFreeze handle global expect stopped orchestration from idle
//
// local thawed => freezing to reach frozen
// else         => stopping
func (o *smon) doFreezeStop() {
	if o.instStatus[o.localhost].Frozen.IsZero() {
		o.doTransitionAction(o.freeze, statusFreezing, statusFrozen, statusFreezeFailed)
		return
	} else {
		o.doStop()
	}
}

func (o *smon) doFreeze() {
	if o.instStatus[o.localhost].Frozen.IsZero() {
		o.doTransitionAction(o.freeze, statusFreezing, statusFrozen, statusFreezeFailed)
		return
	}
}

func (o *smon) doStop() {
	if o.stoppedClearIfReached() {
		return
	}
	o.createPendingWithDuration(stopDuration)
	o.doAction(o.crmStop, statusStopping, statusIdle, statusStopFailed)
}

func (o *smon) stoppedFromReady() {
	o.log.Info().Msg("reset ready state global expect is stopped")
	o.clearPending()
	o.change = true
	o.state.Status = statusIdle
	o.stoppedClearIfReached()
}

func (o *smon) stoppedFromFailed() {
	o.log.Info().Msg("reset %s state global expect is stopped")
	o.change = true
	o.state.Status = statusIdle
	o.stoppedClearIfReached()
}

func (o *smon) stoppedFromAny() {
	if o.pendingCancel == nil {
		o.stoppedClearIfReached()
		return
	}
}

func (o *smon) stoppedClearIfReached() bool {
	if o.isLocalStopped() {
		o.loggerWithState().Info().Msg("local status is stopped, unset global expect")
		o.change = true
		o.state.GlobalExpect = globalExpectUnset
		if o.state.LocalExpect != statusIdle {
			o.state.LocalExpect = statusIdle
		}
		if o.state.Status != statusIdle {
			o.state.Status = statusIdle
		}
		o.clearPending()
		return true
	}
	return false
}

func (o *smon) isLocalStopped() bool {
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
