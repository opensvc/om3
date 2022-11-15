package nmon

import (
	"opensvc.com/opensvc/daemon/daemondata"
)

func (o *nmon) orchestrateThawed() {
	if !o.isConvergedGlobalExpect() {
		return
	}
	switch o.state.Status {
	case statusIdle:
		o.ThawedFromIdle()
	}
}

func (o *nmon) ThawedFromIdle() {
	if o.thawedClearIfReached() {
		return
	}
	o.state.Status = statusThawing
	o.updateIfChange()
	o.log.Info().Msg("run action unfreeze")
	nextState := statusIdle
	if err := o.crmUnfreeze(); err != nil {
		nextState = statusThawedFailed
	}
	go o.orchestrateAfterAction(statusThawing, nextState)
	return
}

func (o *nmon) thawedClearIfReached() bool {
	if d := daemondata.GetNodeStatus(o.dataCmdC, o.localhost); (d != nil) && d.Frozen.IsZero() {
		o.log.Info().Msg("local status is thawed, unset global expect")
		o.change = true
		o.state.GlobalExpect = globalExpectUnset
		o.clearPending()
		return true
	}
	return false
}
