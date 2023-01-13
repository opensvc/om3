package nmon

import (
	"opensvc.com/opensvc/core/cluster"
)

func (o *nmon) orchestrateFrozen() {
	switch o.state.State {
	case cluster.NodeMonitorStateIdle:
		o.frozenFromIdle()
	}
}

func (o *nmon) frozenFromIdle() {
	if o.frozenClearIfReached() {
		return
	}
	o.state.State = cluster.NodeMonitorStateFreezing
	o.updateIfChange()
	o.log.Info().Msg("run action freeze")
	nextState := cluster.NodeMonitorStateIdle
	if err := o.crmFreeze(); err != nil {
		nextState = cluster.NodeMonitorStateFreezeFailed
	}
	go o.orchestrateAfterAction(cluster.NodeMonitorStateFreezing, nextState)
	return
}

func (o *nmon) frozenClearIfReached() bool {
	if d := o.databus.GetNodeStatus(o.localhost); (d != nil) && !d.Frozen.IsZero() {
		o.log.Info().Msg("instance state is frozen, unset global expect")
		o.change = true
		o.state.GlobalExpect = cluster.NodeMonitorGlobalExpectUnset
		o.clearPending()
		return true
	}
	return false
}
