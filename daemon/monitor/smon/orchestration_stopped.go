package smon

import (
	"time"

	"opensvc.com/opensvc/core/status"
)

var (
	stopDuration = 10 * time.Second
)

func (o *smon) orchestrateStopped() {
	if !o.acceptStoppedOrchestration() {
		o.log.Warn().Msg("no solution for orchestrate stopped")
		return
	}
	o.freezeStop()
}

func (o *smon) freezeStop() {
	switch o.state.Status {
	case statusIdle:
		o.doFreezeStop()
	case statusFrozen:
		o.doStop()
	case statusFreezing:
	case statusReady:
		o.stoppedFromReady()
	case statusStopping:
	case statusStopFailed:
		o.stoppedFromFailed()
	case statusStartFailed:
		o.stoppedFromFailed()
	default:
		o.log.Error().Msgf("don't know how to freeze and stop from %s", o.state.Status)
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
	case status.Down:
		return true
	case status.StandbyDown:
		return true
	default:
		return false
	}
}

func (o *smon) acceptStoppedOrchestration() bool {
	switch o.svcAgg.Avail {
	case status.Down:
		return true
	case status.Up:
		return true
	}
	return false
}
