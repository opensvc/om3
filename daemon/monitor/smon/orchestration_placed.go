package smon

import "opensvc.com/opensvc/core/placement"

func (o *smon) orchestratePlaced() {
	if o.state.IsLeader {
		o.orchestratePlacedStart()
	} else {
		o.orchestratePlacedStop()
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
