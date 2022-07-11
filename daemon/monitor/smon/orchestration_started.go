package smon

import (
	"context"
	"time"

	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
)

var (
	readyDuration = 5 * time.Second
)

func (o *smon) orchestrateStarted() {
	if !o.acceptStartedOrchestration() {
		o.log.Warn().Msg("no solution for orchestrate started")
		return
	}
	if !o.isConvergedGlobalExpect() {
		return
	}
	if !o.instStatus[o.localhost].Frozen.IsZero() {
		o.startedFromFrozen()
		return
	}
	switch o.state.Status {
	case statusIdle:
		o.startedFromIdle()
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
// local started => unset global expect, set local expect started
// svcagg.Avail Up => unset global expect, unset local expect
// better candidate => no actions
// else => state -> ready, start ready routine
func (o *smon) startedFromIdle() {
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
	o.change = true
	o.state.Status = statusReady
	o.createPendingWithDuration(readyDuration)
	go func(ctx context.Context) {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				return
			}
			go func() {
				o.cmdC <- moncmd.New(cmdOrchestrate{})
			}()
			return
		}
	}(o.pendingCtx)
}

func (o *smon) startedFromFrozen() {
	o.change = true
	o.state.Status = statusThawing
	go func() {
		o.log.Info().Msg("run action unfreeze")
		if err := o.crmUnfreeze(); err != nil {
			o.cmdC <- moncmd.New(cmdOrchestrate{state: statusThawing, newState: statusThawedFailed})
		} else {
			o.cmdC <- moncmd.New(cmdOrchestrate{state: statusThawing, newState: statusIdle})
		}
	}()
}

func (o *smon) startedFromReady() {
	if o.pendingCancel == nil {
		o.log.Error().Msg("startedFromReady without pending")
		o.change = true
		o.state.Status = statusIdle
		return
	}
	if o.startedClearIfReached() {
		return
	}
	if o.hasBetterCandidateForStarted() {
		o.log.Info().Msg("another better candidate exists, leave ready state")
		o.change = true
		o.state.Status = statusIdle
		o.clearPending()
		return
	}
	select {
	case <-o.pendingCtx.Done():
		o.change = true
		defer o.clearPending()
		if o.pendingCtx.Err() == context.Canceled {
			o.state.Status = statusIdle
			return
		}
		o.state.Status = statusStarting
		o.updateIfChange()
		go func() {
			o.log.Info().Msg("run action start")
			if err := o.crmStart(); err != nil {
				o.cmdC <- moncmd.New(cmdOrchestrate{state: statusStarting, newState: statusStartFailed})
			} else {
				o.cmdC <- moncmd.New(cmdOrchestrate{state: statusStarting, newState: statusIdle})
			}
		}()
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
		o.log.Info().Msg("clear start failed (aggregated status is up)")
		o.change = true
		o.state.GlobalExpect = globalExpectUnset
		o.state.Status = statusIdle
		return
	}
}

func (o *smon) startedClearIfReached() bool {
	if o.isLocalStarted() {
		o.log.Info().Msg("local status is started, unset global expect")
		o.change = true
		o.state.Status = statusIdle
		o.state.GlobalExpect = globalExpectUnset
		if o.state.LocalExpect != statusStarted {
			o.state.LocalExpect = statusStarted
		}
		o.clearPending()
		return true
	}
	if o.svcAgg.Avail == status.Up {
		o.log.Info().Msg("aggregated status is up, unset global expect")
		o.change = true
		o.state.GlobalExpect = globalExpectUnset
		o.state.Status = statusIdle
		o.clearPending()
		return true
	}
	return false
}

func (o *smon) hasBetterCandidateForStarted() bool {
	// TODO change rule (scope from cfg is not for this)
	for node, otherSmon := range o.instSmon {
		if node == o.localhost {
			continue
		}
		switch otherSmon.Status {
		case statusReady:
			if node == o.scopeNodes[0] {
				return true
			}
		case statusStarting:
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
