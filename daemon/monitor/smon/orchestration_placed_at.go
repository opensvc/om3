package smon

import (
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/topology"
)

func (o *smon) orchestratePlacedAt(dst string) {
	dstNodes := o.parsePlacedAtDestination(dst)
	if dstNodes.Contains(o.localhost) {
		o.orchestratePlacedStart()
	} else {
		o.orchestratePlacedStop()
	}
}

func (o *smon) doPlacedStart() {
	if instStatus, ok := o.instStatus[o.localhost]; ok && instStatus.Topology == topology.Failover {
		// failover objects need to wait for the agg status to reach "down"
		switch o.svcAgg.Avail {
		case status.Down:
		default:
			return
		}
	}
	o.doAction(o.crmStart, statusStarting, statusStarted, statusStartFailed)
}

func (o *smon) doPlacedStop() {
	o.createPendingWithDuration(stopDuration)
	o.doAction(o.crmStop, statusStopping, statusStopped, statusStopFailed)
}

func (o *smon) clearStoppedIfAggUp() {
	switch o.svcAgg.Avail {
	case status.Up:
		o.loggerWithState().Info().Msg("agg status is up, unset global expect")
		o.change = true
		o.state.GlobalExpect = globalExpectUnset
		if o.state.LocalExpect != statusIdle {
			o.state.LocalExpect = statusIdle
		}
		if o.state.Status != statusIdle {
			o.state.Status = statusIdle
		}
		o.clearPending()
	}
}

func (o *smon) orchestratePlacedStart() {
	switch o.state.Status {
	case statusStarted:
		o.startedClearIfReached()
	case statusStopped, statusIdle:
		o.doPlacedStart()
	}
}

func (o *smon) orchestratePlacedStop() {
	if !o.acceptStoppedOrchestration() {
		o.log.Warn().Msg("no solution for orchestrate stopped")
		return
	}
	switch o.state.Status {
	case statusIdle:
		o.doPlacedStop()
	case statusFreezing:
	case statusReady:
		o.stoppedFromReady()
	case statusStopping:
	case statusStopped:
		o.clearStoppedIfAggUp()
	case statusStopFailed:
		o.transitionTo(statusIdle)
	case statusStartFailed:
		o.transitionTo(statusIdle)
	default:
		o.log.Error().Msgf("don't know how to orchestrate stopped from %s", o.state.Status)
	}
}

func (o *smon) placedStopFromReady() {
	o.log.Info().Msg("reset ready state global expect is placed")
	o.clearPending()
	o.transitionTo(statusStopped)
}
