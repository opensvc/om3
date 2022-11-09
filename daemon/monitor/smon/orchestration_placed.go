package smon

import "opensvc.com/opensvc/core/placement"

func (o *smon) orchestratePlaced() {
	if !o.acceptPlacedOrchestration() {
		o.log.Warn().Msg("no solution for orchestrate placed")
		return
	}
	switch o.state.Status {
	default:
		o.log.Error().Msgf("don't know how to orchestrate placed from %s", o.state.Status)
	}
}

//func (o *smon) stoppedFromThawed() {
//	o.doAction(o.crmFreeze, statusFreezing, statusIdle, statusFreezeFailed)
//}

func (o *smon) acceptPlacedOrchestration() bool {
	switch o.svcAgg.Placement {
	case placement.Optimal:
		return false
	case placement.NotApplicable:
		return false
	}
	return true
}
