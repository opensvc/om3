package smon

import "opensvc.com/opensvc/core/placement"

func (o *smon) orchestratePlaced() {
	if o.state.IsHALeader {
		o.orchestratePlacedStart()
	} else {
		o.orchestratePlacedStop()
	}
}

func (o *smon) acceptPlacedOrchestration() bool {
	switch o.svcAgg.PlacementState {
	case placement.Optimal:
		return false
	case placement.NotApplicable:
		return false
	}
	return true
}
