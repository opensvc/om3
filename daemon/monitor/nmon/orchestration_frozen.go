package nmon

import "opensvc.com/opensvc/core/node"

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
	if d := o.databus.GetNodeStatus(o.localhost); (d != nil) && !d.Frozen.IsZero() {
		o.log.Info().Msg("instance state is frozen, unset global expect")
		o.change = true
		o.state.GlobalExpect = node.MonitorGlobalExpectUnset
		o.clearPending()
		return true
	}
	return false
}
