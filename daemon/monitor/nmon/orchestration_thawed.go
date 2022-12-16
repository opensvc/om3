package nmon

import (
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/daemondata"
)

func (o *nmon) orchestrateThawed() {
	if !o.isConvergedGlobalExpect() {
		return
	}
	switch o.state.Status {
	case cluster.NodeMonitorStatusIdle:
		o.ThawedFromIdle()
	}
}

func (o *nmon) ThawedFromIdle() {
	if o.thawedClearIfReached() {
		return
	}
	o.state.Status = cluster.NodeMonitorStatusThawing
	o.updateIfChange()
	o.log.Info().Msg("run action unfreeze")
	nextState := cluster.NodeMonitorStatusIdle
	if err := o.crmUnfreeze(); err != nil {
		nextState = cluster.NodeMonitorStatusThawedFailed
	}
	go o.orchestrateAfterAction(cluster.NodeMonitorStatusThawing, nextState)
	return
}

func (o *nmon) thawedClearIfReached() bool {
	if d := daemondata.GetNodeStatus(o.dataCmdC, o.localhost); (d != nil) && d.Frozen.IsZero() {
		o.log.Info().Msg("local status is thawed, unset global expect")
		o.change = true
		o.state.GlobalExpect = cluster.NodeMonitorGlobalExpectUnset
		o.clearPending()
		return true
	}
	return false
}
