package imon

import (
	"github.com/opensvc/om3/core/instance"
)

func (o *imon) orchestrateThawed() {
	switch o.state.State {
	case instance.MonitorStateIdle,
		instance.MonitorStateStartFailed,
		instance.MonitorStateStopFailed,
		instance.MonitorStatePurgeFailed,
		instance.MonitorStateProvisionFailed,
		instance.MonitorStateUnprovisionFailed,
		instance.MonitorStateReady:
		o.ThawedFromIdle()
	}
}

func (o *imon) ThawedFromIdle() {
	if o.thawedClearIfReached() {
		return
	}
	o.doTransitionAction(o.unfreeze, instance.MonitorStateThawing, instance.MonitorStateIdle, instance.MonitorStateThawedFailed)
}

func (o *imon) thawedClearIfReached() bool {
	if o.instStatus[o.localhost].IsThawed() {
		o.log.Info().Msg("instance state is thawed -> set reached, clear local expect")
		o.setReached()
		o.state.LocalExpect = instance.MonitorLocalExpectNone
		o.clearPending()
		return true
	}
	return false
}
