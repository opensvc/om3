package nmon

import "github.com/opensvc/om3/core/node"

func (o *nmon) orchestrateThawed() {
	switch o.state.State {
	case node.MonitorStateIdle:
		o.ThawedFromIdle()
	case node.MonitorStateThawing:
	default:
		o.log.Warn().Msgf("daemon: nmon: don't know how to orchestrate %s from %s", o.state.GlobalExpect, o.state.State)
	}
}

func (o *nmon) ThawedFromIdle() {
	if o.thawedClearIfReached() {
		return
	}
	o.transitionTo(node.MonitorStateThawing)
	o.log.Info().Msg("daemon: nmon: run action unfreeze")
	nextState := node.MonitorStateIdle
	if err := o.crmUnfreeze(); err != nil {
		nextState = node.MonitorStateThawedFailed
	}
	go o.orchestrateAfterAction(node.MonitorStateThawing, nextState)
	return
}

func (o *nmon) thawedClearIfReached() bool {
	if nodeStatus := node.StatusData.Get(o.localhost); nodeStatus != nil && nodeStatus.FrozenAt.IsZero() {
		o.log.Info().Msg("daemon: nmon: instance state is thawed, unset global expect")
		o.change = true
		o.state.GlobalExpect = node.MonitorGlobalExpectNone
		o.clearPending()
		return true
	}
	return false
}
