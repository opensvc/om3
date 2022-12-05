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
	if !o.nodeStatus.Frozen.IsZero() {
		return
	}
	instStatus := o.instStatus[o.localhost]
	if instStatus.Orchestrate != "ha" {
		return
	}
	o.orchestrateStarted()
}

func (o *smon) orchestrateStarted() {
	if !o.acceptStartedOrchestration() {
		o.log.Warn().Msg("no solution for orchestrate started")
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
	if o.hasBetterCandidateForStarted() {
		o.log.Debug().Msg("better candidate found for started")
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
	if o.hasBetterCandidateForStarted() {
		o.loggerWithState().Info().Msg("another better candidate exists, leave ready state")
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
	if o.svcAgg.Avail == status.Up {
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
	if o.svcAgg.Avail == status.Up {
		if o.state.Status != statusIdle {
			o.loggerWithState().Info().Msg("aggregated status is up, unset status")
			o.change = true
			o.state.Status = statusIdle
		}
		if o.state.GlobalExpect != globalExpectUnset {
			o.loggerWithState().Info().Msg("aggregated status is up, unset global expect")
			o.change = true
			o.state.GlobalExpect = globalExpectUnset
		}
		o.clearPending()
		return true
	}
	return false
}

func (o *smon) hasBetterCandidateForStarted() bool {
	for node, otherSmon := range o.instSmon {
		if node == o.localhost {
			continue
		}
		switch otherSmon.Status {
		case statusReady:
			if otherSmon.IsLeader {
				return true
			}
		case statusStarting:
			return true
		case statusStarted:
			return true
		}
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

func (o *smon) acceptStartedOrchestration() bool {
	switch o.svcAgg.Avail {
	case status.Down:
		return true
	case status.Up:
		return true
	}
	return false
}
