package smon

import (
	"opensvc.com/opensvc/daemon/monitor/moncmd"
)

func (o *smon) orchestrateProvisioned() {
	if !o.isConvergedGlobalExpect() {
		return
	}
	switch o.state.Status {
	case statusIdle:
		o.provisionedFromIdle()
	}
}

func (o *smon) provisionedFromIdle() {
	if o.provisionedClearIfReached() {
		return
	}
	o.change = true
	o.state.Status = statusProvisioning
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run action provision")
		if err := o.crmProvisionLeader(); err != nil {
			o.cmdC <- moncmd.New(cmdOrchestrate{state: statusProvisioning, newState: statusProvisionFailed})
		} else {
			o.cmdC <- moncmd.New(cmdOrchestrate{state: statusProvisioning, newState: statusIdle})
		}
	}()
	return
}

func (o *smon) provisionedClearIfReached() bool {
	if o.instStatus[o.localhost].Provisioned.Bool() {
		o.log.Info().Msg("local is already provisioned, unset global expect")
		o.change = true
		o.state.GlobalExpect = globalExpectUnset
		if o.state.LocalExpect != statusIdle {
			o.state.LocalExpect = statusIdle
		}
		return true
	}
	return false
}
