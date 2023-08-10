package imon

import (
	"context"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
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
	case instance.MonitorStateStarted:
		o.startedFromStarted()
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
		if o.state.GlobalExpect == instance.MonitorGlobalExpectNone {
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
	if o.instStatus[o.localhost].Provisioned.IsOneOf(provisioned.False, provisioned.Undef) {
		o.log.Debug().Msg("provisioned is false or undef")
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
		o.loggerWithState().Info().Msg("leadership lost, clear the ready state")
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
		o.doAction(o.crmStart, instance.MonitorStateStarting, instance.MonitorStateStarted, instance.MonitorStateStartFailed)
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

func (o *imon) startedFromStarted() {
	o.startedClearIfReached()
}

func (o *imon) startedFromStartFailed() {
	if o.isStarted() {
		o.loggerWithState().Info().Msg("object is up -> set done and idle, clear start failed")
		o.doneAndIdle()
		return
	}
}

func (o *imon) startedClearIfReached() bool {
	if o.isLocalStarted() {
		if !o.state.OrchestrationIsDone {
			o.loggerWithState().Info().Msg("instance is started -> set done and idle")
			o.doneAndIdle()
		}
		if o.state.LocalExpect != instance.MonitorLocalExpectStarted {
			o.loggerWithState().Info().Msg("instance is started, set local expect started")
			o.change = true
			o.state.LocalExpect = instance.MonitorLocalExpectStarted
		}
		o.clearPending()
		return true
	}
	if o.isStarted() {
		if !o.state.OrchestrationIsDone {
			o.loggerWithState().Info().Msg("object is started -> set done and idle")
			o.doneAndIdle()
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
