package nmon

import (
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
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
	go func() {
		o.log.Info().Msg("run action unfreeze")
		if err := o.crmUnfreeze(); err != nil {
			o.cmdC <- moncmd.New(cmdOrchestrate{state: statusThawing, newState: statusThawedFailed})
		} else {
			o.cmdC <- moncmd.New(cmdOrchestrate{state: statusThawing, newState: statusIdle})
		}
	}()
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
