package imon

import (
	"context"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/status"
)

func (o *imon) orchestrateStarted() {
	if o.isStarted() {
		o.startedClearIfReached()
		return
	}
	switch o.state.State {
	case instance.MonitorStateIdle:
		o.startedFromIdle()
	case instance.MonitorStateThawed:
		o.startedFromThawed()
	case instance.MonitorStateReady:
		o.startedFromReady()
	case instance.MonitorStateStartFailed:
		o.startedFromStartFailed()
	case instance.MonitorStateStarting:
		o.startedFromAny()
	case instance.MonitorStateStopping:
		o.startedFromAny()
	case instance.MonitorStateThawing:
	default:
		o.log.Error().Msgf("don't know how to orchestrate started from %s", o.state.State)
	}
}

// startedFromIdle handle global expect started orchestration from idle
//
// frozen => try startedFromFrozen
// else   => try startedFromThawed
func (o *imon) startedFromIdle() {
	if o.instStatus[o.localhost].IsFrozen() {
		if o.state.GlobalExpect == instance.MonitorGlobalExpectUnset {
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
// objectStatus.Avail Up => unset global expect, unset local expect
// better candidate => no actions
// else => state -> ready, start ready routine
func (o *imon) startedFromThawed() {
	if o.startedClearIfReached() {
		return
	}
	if !o.state.IsHALeader {
		return
	}
	if o.hasOtherNodeActing() {
		o.log.Debug().Msg("another node acting")
		return
	}
	o.transitionTo(instance.MonitorStateReady)
	o.createPendingWithDuration(o.readyDuration)
	go func(ctx context.Context) {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				return
			}
			o.orchestrateAfterAction(instance.MonitorStateReady, instance.MonitorStateReady)
			return
		}
	}(o.pendingCtx)
}

// doUnfreeze idle -> thawing -> thawed or thawed failed
func (o *imon) doUnfreeze() {
	o.doTransitionAction(o.unfreeze, instance.MonitorStateThawing, instance.MonitorStateThawed, instance.MonitorStateThawedFailed)
}

func (o *imon) cancelReadyState() bool {
	if o.pendingCancel == nil {
		o.loggerWithState().Error().Msg("startedFromReady without pending")
		o.transitionTo(instance.MonitorStateIdle)
		return true
	}
	if o.startedClearIfReached() {
		return true
	}
	if !o.state.IsHALeader {
		o.loggerWithState().Info().Msg("leadership lost, leave ready state")
		o.transitionTo(instance.MonitorStateIdle)
		o.clearPending()
		return true
	}
	return false
}

func (o *imon) startedFromReady() {
	if isCanceled := o.cancelReadyState(); isCanceled {
		return
	}
	select {
	case <-o.pendingCtx.Done():
		defer o.clearPending()
		if o.pendingCtx.Err() == context.Canceled {
			o.transitionTo(instance.MonitorStateIdle)
			return
		}
		o.doAction(o.crmStart, instance.MonitorStateStarting, instance.MonitorStateIdle, instance.MonitorStateStartFailed)
		return
	default:
		return
	}
}

func (o *imon) startedFromAny() {
	if o.pendingCancel == nil {
		o.startedClearIfReached()
		return
	}
}

func (o *imon) startedFromStartFailed() {
	if o.isStarted() {
		o.loggerWithState().Info().Msg("clear start failed (object is up)")
		o.change = true
		o.state.GlobalExpect = instance.MonitorGlobalExpectUnset
		o.state.State = instance.MonitorStateIdle
		return
	}
}

func (o *imon) startedClearIfReached() bool {
	if o.isLocalStarted() {
		if o.state.State != instance.MonitorStateIdle {
			o.loggerWithState().Info().Msg("instance is started, unset state")
			o.change = true
			o.state.State = instance.MonitorStateIdle
		}
		if o.state.GlobalExpect != instance.MonitorGlobalExpectUnset {
			o.loggerWithState().Info().Msg("instance is started, unset global expect")
			o.change = true
			o.state.GlobalExpect = instance.MonitorGlobalExpectUnset
		}
		if o.state.LocalExpect != instance.MonitorLocalExpectStarted {
			o.loggerWithState().Info().Msg("instance is started, unset local expect")
			o.change = true
			o.state.LocalExpect = instance.MonitorLocalExpectStarted
		}
		o.clearPending()
		return true
	}
	if o.isStarted() {
		if o.state.State != instance.MonitorStateIdle {
			o.loggerWithState().Info().Msg("object is started, unset status")
			o.change = true
			o.state.State = instance.MonitorStateIdle
		}
		if o.state.GlobalExpect != instance.MonitorGlobalExpectUnset {
			o.loggerWithState().Info().Msg("object is started, unset global expect")
			o.change = true
			o.state.GlobalExpect = instance.MonitorGlobalExpectUnset
		}
		o.clearPending()
		return true
	}
	return false
}

func (o *imon) isLocalStarted() bool {
	instStatus := o.instStatus[o.localhost]
	switch instStatus.Avail {
	case status.NotApplicable:
		return true
	case status.Up:
		return true
	case status.StandbyUp:
		return true
	case status.Undef:
		return false
	default:
		return false
	}
}
