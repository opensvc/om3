package imon

import (
	"github.com/opensvc/om3/core/instance"
)

func (o *imon) orchestrateThawed() {
	switch o.state.State {
	case instance.MonitorStateIdle,
		instance.MonitorStateStartFailed,
		instance.MonitorStateStarted,
		instance.MonitorStateStopFailed,
		instance.MonitorStateStopped,
		instance.MonitorStatePurgeFailed,
		instance.MonitorStateProvisionFailed,
		instance.MonitorStateProvisioned,
		instance.MonitorStateUnprovisionFailed,
		instance.MonitorStateUnprovisioned,
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
		o.doneAndIdle()
		o.state.LocalExpect = instance.MonitorLocalExpectNone
		o.clearPending()
		return true
	}
	return false
}
