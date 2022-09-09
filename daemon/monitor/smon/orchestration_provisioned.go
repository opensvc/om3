package smon

import (
	"time"

	"opensvc.com/opensvc/daemon/msgbus"
)

func (o *smon) orchestrateProvisioned() {
	if !o.isConvergedGlobalExpect() {
		return
	}
	switch o.state.Status {
	case statusIdle:
		o.provisionedFromIdle()
	case statusWaitLeader:
		o.provisionedFromWaitLeader()
	}
}

func (o *smon) provisionedFromIdle() {
	if o.provisionedClearIfReached() {
		return
	}
	if o.isProvisioningLeader() {
		o.change = true
		o.state.Status = statusProvisioning
		o.updateIfChange()
		go func() {
			o.log.Info().Msg("run action provision leader for provisioned global expect")
			if err := o.crmProvisionLeader(); err != nil {
				o.cmdC <- msgbus.NewMsg(cmdOrchestrate{state: statusProvisioning, newState: statusProvisionFailed})
			} else {
				o.cmdC <- msgbus.NewMsg(cmdOrchestrate{state: statusProvisioning, newState: statusIdle})
				// TODO remove o.crmStatus() when updated is valid
				go func() {
					time.Sleep(time.Second)
					o.crmStatus()
				}()
			}
		}()
		return
	}
	o.change = true
	o.state.Status = statusWaitLeader
	o.updateIfChange()
}

func (o *smon) provisionedFromWaitLeader() {
	if o.provisionedClearIfReached() {
		return
	}
	if !o.hasLeaderProvisioned() {
		return
	}
	o.change = true
	o.state.Status = statusProvisioning
	o.updateIfChange()
	go func() {
		o.log.Info().Msg("run action provision non leader for provisioned global expect")
		if err := o.crmProvision(); err != nil {
			o.cmdC <- msgbus.NewMsg(cmdOrchestrate{state: statusProvisioning, newState: statusProvisionFailed})
		} else {
			o.cmdC <- msgbus.NewMsg(cmdOrchestrate{state: statusProvisioning, newState: statusIdle})
			// TODO remove o.crmStatus() when updated is valid
			go func() {
				time.Sleep(time.Second)
				o.crmStatus()
			}()
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
		o.updateIfChange()
		return true
	}
	return false
}

func (o *smon) isProvisioningLeader() bool {
	if o.scopeNodes[0] == o.localhost {
		return true
	}
	return false
}

func (o *smon) hasLeaderProvisioned() bool {
	// TODO change rule (scope from cfg is not for this)
	leader := o.scopeNodes[0]
	if leaderInstanceStatus, ok := o.instStatus[leader]; !ok {
		return false
	} else if !leaderInstanceStatus.Provisioned.Bool() {
		return false
	}
	return true
}
