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
	if o.svcAgg.Topology == topology.Failover {
		// failover objects need to wait for the agg status to reach "down"
		switch o.svcAgg.Avail {
		case status.Down:
		default:
			return
		}
	}
	o.doAction(o.crmStart, statusStarting, statusStarted, statusStartFailed)
}

func (o *smon) placedStop() {
	instStatus := o.instStatus[o.localhost]
	switch instStatus.Avail {
	case status.Down, status.StandbyDown, status.StandbyUp:
		o.placedStopFromDown()
	case status.Up, status.Warn:
		o.doPlacedStop()
	default:
		return
	}
}

func (o *smon) doPlacedStop() {
	o.createPendingWithDuration(stopDuration)
	o.doAction(o.crmStop, statusStopping, statusStopped, statusStopFailed)
}

func (o *smon) placedStopFromDown() {
	o.loggerWithState().Info().Msg("instance is already down")
	o.change = true
	o.state.Status = statusStopped
	o.clearPending()
}

func (o *smon) clearStopFailedIfDown() {
	instStatus := o.instStatus[o.localhost]
	switch instStatus.Avail {
	case status.Down, status.StandbyDown:
		o.loggerWithState().Info().Msg("instance is down, clear stop failed")
		o.change = true
		o.state.Status = statusStopped
		o.clearPending()
	}
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
		o.log.Warn().Msg("no solution for orchestrate placed stopped")
		return
	}
	switch o.state.Status {
	case statusIdle:
		o.doPlacedStop()
	case statusFreezing:
	case statusReady:
		o.stoppedFromReady()
	case statusStopFailed:
		o.clearStopFailedIfDown()
	case statusStopping:
	case statusStopped:
		o.clearStoppedIfAggUp()
	case statusStartFailed:
		o.transitionTo(statusIdle)
	default:
		o.log.Error().Msgf("don't know how to orchestrate placed stopped from %s", o.state.Status)
	}
}

func (o *smon) placedStopFromReady() {
	o.log.Info().Msg("reset ready state global expect is placed")
	o.clearPending()
	o.transitionTo(statusStopped)
}
