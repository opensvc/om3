package smon

import (
	"time"

	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
)

var (
	stopDuration = 10 * time.Second
)

func (o *smon) orchestrateStopped() {
	if !o.acceptStoppedOrchestration() {
		o.log.Warn().Msg("no solution for orchestrate stopped")
		return
	}
	if !o.isConvergedGlobalExpect() {
		return
	}
	switch o.state.Status {
	case statusIdle:
		o.stoppedFromIdle()
	case statusStopping:
	default:
		o.log.Error().Msgf("don't know how to orchestrate stopped from %s", o.state.Status)
	}
}

// stoppedFromIdle handle global expect stopped orchestration from idle
//
// local stopped => unset global expect, unset local expect
// else => state -> stopping, start stop routine
func (o *smon) stoppedFromIdle() {
	if o.stoppedClearIfReached() {
		return
	}
	o.change = true
	o.state.Status = statusStopping
	o.updateIfChange()
	o.createPendingWithDuration(stopDuration)
	go func() {
		o.log.Info().Msg("run action stop")
		if err := o.crmStop(); err != nil {
			o.cmdC <- moncmd.New(cmdOrchestrate{state: statusStopping, newState: statusStopFailed})
		} else {
			o.cmdC <- moncmd.New(cmdOrchestrate{state: statusStopping, newState: statusIdle})
		}
	}()
}

func (o *smon) stoppedFromAny() {
	if o.pendingCancel == nil {
		o.stoppedClearIfReached()
		return
	}
}

func (o *smon) stoppedClearIfReached() bool {
	if o.isLocalStopped() {
		o.log.Info().Msg("local status is stopped, unset global expect")
		o.change = true
		o.state.GlobalExpect = globalExpectUnset
		if o.state.LocalExpect != statusIdle {
			o.state.LocalExpect = statusIdle
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
