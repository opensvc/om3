package imon

import "opensvc.com/opensvc/core/instance"

func (o *imon) orchestrateFrozen() {
	switch o.state.State {
	case instance.MonitorStateIdle:
		o.frozenFromIdle()
	}
}

func (o *imon) frozenFromIdle() {
	if o.frozenClearIfReached() {
		return
	}
	o.doTransitionAction(o.freeze, instance.MonitorStateFreezing, instance.MonitorStateIdle, instance.MonitorStateFreezeFailed)
}

func (o *imon) frozenClearIfReached() bool {
	if o.instStatus[o.localhost].IsFrozen() {
		o.log.Info().Msg("local status is frozen, unset global expect")
		o.change = true
		o.state.GlobalExpect = instance.MonitorGlobalExpectUnset
		o.state.LocalExpect = instance.MonitorLocalExpectUnset
		o.clearPending()
		return true
	}
	return false
}
