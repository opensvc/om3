package nmon

import (
	"opensvc.com/opensvc/core/cluster"
)

func (o *nmon) orchestrateThawed() {
	switch o.state.State {
	case cluster.NodeMonitorStateIdle:
		o.ThawedFromIdle()
	default:
		o.log.Warn().Msgf("don't know how to orchestrate %s from %s", o.state.GlobalExpect, o.state.State)
	}
}

func (o *nmon) ThawedFromIdle() {
	if o.thawedClearIfReached() {
		return
	}
	o.transitionTo(cluster.NodeMonitorStateThawing)
	o.log.Info().Msg("run action unfreeze")
	nextState := cluster.NodeMonitorStateIdle
	if err := o.crmUnfreeze(); err != nil {
		nextState = cluster.NodeMonitorStateThawedFailed
	}
	go o.orchestrateAfterAction(cluster.NodeMonitorStateThawing, nextState)
	return
}

func (o *nmon) thawedClearIfReached() bool {
	if d := o.databus.GetNodeStatus(o.localhost); (d != nil) && d.Frozen.IsZero() {
		o.log.Info().Msg("local status is thawed, unset global expect")
		o.change = true
		o.state.GlobalExpect = cluster.NodeMonitorGlobalExpectUnset
		o.clearPending()
		return true
	}
	return false
}
