package imon

import "opensvc.com/opensvc/core/instance"

func (o *imon) orchestrateThawed() {
	switch o.state.State {
	case instance.MonitorStateIdle:
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
		o.log.Info().Msg("local status is thawed, unset global expect")
		o.change = true
		o.state.GlobalExpect = instance.MonitorGlobalExpectUnset
		o.state.LocalExpect = instance.MonitorLocalExpectUnset
		o.clearPending()
		return true
	}
	return false
}
