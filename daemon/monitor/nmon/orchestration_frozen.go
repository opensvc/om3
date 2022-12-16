package nmon

import (
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/daemondata"
)

func (o *nmon) orchestrateFrozen() {
	if !o.isConvergedGlobalExpect() {
		return
	}
	switch o.state.Status {
	case cluster.NodeMonitorStatusIdle:
		o.frozenFromIdle()
	}
}

func (o *nmon) frozenFromIdle() {
	if o.frozenClearIfReached() {
		return
	}
	o.state.Status = cluster.NodeMonitorStatusFreezing
	o.updateIfChange()
	o.log.Info().Msg("run action freeze")
	nextState := cluster.NodeMonitorStatusIdle
	if err := o.crmFreeze(); err != nil {
		nextState = cluster.NodeMonitorStatusFreezeFailed
	}
	go o.orchestrateAfterAction(cluster.NodeMonitorStatusFreezing, nextState)
	return
}

func (o *nmon) frozenClearIfReached() bool {
	if d := daemondata.GetNodeStatus(o.dataCmdC, o.localhost); (d != nil) && !d.Frozen.IsZero() {
		o.log.Info().Msg("local status is frozen, unset global expect")
		o.change = true
		o.state.GlobalExpect = cluster.NodeMonitorGlobalExpectUnset
		o.clearPending()
		return true
	}
	return false
}
