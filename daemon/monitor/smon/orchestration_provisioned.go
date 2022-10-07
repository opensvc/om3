package smon

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
		o.doAction(o.crmProvisionLeader, statusProvisioning, statusIdle, statusProvisionFailed)
		return
	} else {
		o.transitionTo(statusWaitLeader)
	}
}

func (o *smon) provisionedFromWaitLeader() {
	if o.provisionedClearIfReached() {
		o.transitionTo(statusIdle)
		return
	}
	if !o.hasLeaderProvisioned() {
		return
	}
	o.doAction(o.crmProvisionNonLeader, statusProvisioning, statusIdle, statusProvisionFailed)
	return
}

func (o *smon) provisionedClearIfReached() bool {
	if o.instStatus[o.localhost].Provisioned.Bool() {
		o.log.Info().Msg("global expect provisioned local status provisioned, unset global expect")
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
