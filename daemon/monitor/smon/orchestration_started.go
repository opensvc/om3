package smon

import (
	"context"
	"time"

	"opensvc.com/opensvc/core/status"
)

var (
	readyDuration = 5 * time.Second
)

func (o *smon) orchestrateAutoStarted() {
	nodeStatus := o.nodeStatus[o.localhost]
	if !nodeStatus.Frozen.IsZero() {
		return
	}
	instStatus := o.instStatus[o.localhost]
	if instStatus.Orchestrate != "ha" {
		return
	}
	if v, _ := o.isStartable(); !v {
		return
	}
	o.orchestrateStarted()
}

func (o *smon) orchestrateStarted() {
	if o.isStarted() {
		return
	}
	switch o.state.Status {
	case statusIdle:
		o.startedFromIdle()
	case statusThawed:
		o.startedFromThawed()
	case statusReady:
		o.startedFromReady()
	case statusStartFailed:
		o.startedFromStartFailed()
	case statusStarting:
		o.startedFromAny()
	case statusStopping:
		o.startedFromAny()
	case statusThawing:
	default:
		o.log.Error().Msgf("don't know how to orchestrate started from %s", o.state.Status)
	}
}

// startedFromIdle handle global expect started orchestration from idle
//
// frozen => try startedFromFrozen
// else   => try startedFromThawed
func (o *smon) startedFromIdle() {
	if !o.instStatus[o.localhost].Frozen.IsZero() {
		if o.state.GlobalExpect == globalExpectUnset {
			return
		}
		o.doUnfreeze()
		return
	} else {
		o.startedFromThawed()
	}
}

// startedFromThawed
//
// local started => unset global expect, set local expect started
// svcagg.Avail Up => unset global expect, unset local expect
// better candidate => no actions
// else => state -> ready, start ready routine
func (o *smon) startedFromThawed() {
	if o.startedClearIfReached() {
		return
	}
	if !o.state.IsLeader {
		return
	}
	if o.hasOtherNodeActing() {
		o.log.Debug().Msg("another node acting")
		return
	}
	o.transitionTo(statusReady)
	o.createPendingWithDuration(readyDuration)
	go func(ctx context.Context) {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				return
			}
			o.orchestrateAfterAction("", "")
			return
		}
	}(o.pendingCtx)
}

// doUnfreeze idle -> thawing -> thawed or thawed failed
func (o *smon) doUnfreeze() {
	o.doTransitionAction(o.unfreeze, statusThawing, statusThawed, statusThawedFailed)
}

func (o *smon) startedFromReady() {
	if o.pendingCancel == nil {
		o.loggerWithState().Error().Msg("startedFromReady without pending")
		o.transitionTo(statusIdle)
		return
	}
	if o.startedClearIfReached() {
		return
	}
	if !o.state.IsLeader {
		o.loggerWithState().Info().Msg("leadership lost, leave ready state")
		o.transitionTo(statusIdle)
		o.clearPending()
		return
	}
	select {
	case <-o.pendingCtx.Done():
		defer o.clearPending()
		if o.pendingCtx.Err() == context.Canceled {
			o.transitionTo(statusIdle)
			return
		}
		o.doAction(o.crmStart, statusStarting, statusIdle, statusStartFailed)
		return
	default:
		return
	}
}

func (o *smon) startedFromAny() {
	if o.pendingCancel == nil {
		o.startedClearIfReached()
		return
	}
}

func (o *smon) startedFromStartFailed() {
	if o.isStarted() {
		o.loggerWithState().Info().Msg("clear start failed (aggregated status is up)")
		o.change = true
		o.state.GlobalExpect = globalExpectUnset
		o.state.Status = statusIdle
		return
	}
}

func (o *smon) startedClearIfReached() bool {
	if o.isLocalStarted() {
		if o.state.Status != statusIdle {
			o.loggerWithState().Info().Msg("local status is started, unset status")
			o.change = true
			o.state.Status = statusIdle
		}
		if o.state.GlobalExpect != globalExpectUnset {
			o.loggerWithState().Info().Msg("local status is started, unset global expect")
			o.change = true
			o.state.GlobalExpect = globalExpectUnset
		}
		if o.state.LocalExpect != statusStarted {
			o.loggerWithState().Info().Msg("local status is started, unset local expect")
			o.change = true
			o.state.LocalExpect = statusStarted
		}
		o.clearPending()
		return true
	}
	if o.isStarted() {
		if o.state.Status != statusIdle {
			o.loggerWithState().Info().Msg("object is started, unset status")
			o.change = true
			o.state.Status = statusIdle
		}
		if o.state.GlobalExpect != globalExpectUnset {
			o.loggerWithState().Info().Msg("object is started, unset global expect")
			o.change = true
			o.state.GlobalExpect = globalExpectUnset
		}
		o.clearPending()
		return true
	}
	return false
}

func (o *smon) isLocalStarted() bool {
	instStatus := o.instStatus[o.localhost]
	switch instStatus.Avail {
	case status.Up:
		return true
	case status.StandbyUp:
		return true
	default:
		return false
	}
}
