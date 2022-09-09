package smon

import "opensvc.com/opensvc/daemon/msgbus"

func (o *smon) orchestrateUnProvisioned() {
	if !o.isConvergedGlobalExpect() {
		return
	}
	switch o.state.Status {
	case statusIdle:
		o.UnProvisionedFromIdle()
	}
}

func (o *smon) UnProvisionedFromIdle() {
	if o.UnProvisionedClearIfReached() {
		return
	}
	o.change = true
	o.state.Status = statusUnProvisioning
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run action unprovision")
		if err := o.crmUnprovisionLeader(); err != nil {
			o.cmdC <- msgbus.NewMsg(cmdOrchestrate{state: statusUnProvisioning, newState: statusUnProvisionFailed})
		} else {
			o.cmdC <- msgbus.NewMsg(cmdOrchestrate{state: statusUnProvisioning, newState: statusIdle})
		}
	}()
	return
}

func (o *smon) UnProvisionedClearIfReached() bool {
	if !o.instStatus[o.localhost].Provisioned.Bool() {
		o.log.Info().Msg("local status is not provisioned, unset global expect")
		o.change = true
		o.state.GlobalExpect = globalExpectUnset
		if o.state.LocalExpect != statusIdle {
			o.state.LocalExpect = statusIdle
		}
		return true
	}
	return false
}
