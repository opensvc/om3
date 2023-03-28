package imon

import "github.com/opensvc/om3/core/instance"

func (o *imon) orchestrateFrozen() {
	switch o.state.State {
	case instance.MonitorStateIdle,
		instance.MonitorStateStartFailed,
		instance.MonitorStateStopFailed,
		instance.MonitorStatePurgeFailed,
		instance.MonitorStateProvisionFailed,
		instance.MonitorStateUnprovisionFailed,
		instance.MonitorStateReady:
		o.frozenFromIdle()
	default:
		o.log.Warn().Msgf("orchestrateFrozen has no solution from state %s", o.state.State)
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
		o.log.Info().Msg("instance state is frozen -> set reached, clear local expect")
		o.setReached()
		o.state.LocalExpect = instance.MonitorLocalExpectNone
		o.clearPending()
		return true
	}
	return false
}
