package nmon

import "github.com/opensvc/om3/core/node"

func (o *nmon) orchestrateFrozen() {
	switch o.state.State {
	case node.MonitorStateIdle:
		o.frozenFromIdle()
	}
}

func (o *nmon) frozenFromIdle() {
	if o.frozenClearIfReached() {
		return
	}
	o.state.State = node.MonitorStateFreezing
	o.updateIfChange()
	o.log.Info().Msg("run action freeze")
	nextState := node.MonitorStateIdle
	if err := o.crmFreeze(); err != nil {
		nextState = node.MonitorStateFreezeFailed
	}
	go o.orchestrateAfterAction(node.MonitorStateFreezing, nextState)
	return
}

func (o *nmon) frozenClearIfReached() bool {
	if nodeStatus := node.StatusData.Get(o.localhost); nodeStatus != nil && !nodeStatus.FrozenAt.IsZero() {
		o.log.Info().Msg("instance state is frozen, unset global expect")
		o.change = true
		o.state.GlobalExpect = node.MonitorGlobalExpectNone
		o.clearPending()
		return true
	}
	return false
}
