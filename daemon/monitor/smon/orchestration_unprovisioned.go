package smon

func (o *smon) orchestrateUnProvisioned() {
	if !o.isConvergedGlobalExpect() {
		return
	}
	switch o.state.Status {
	case statusIdle:
		o.UnProvisionedFromIdle()
	case statusWaitNonLeader:
		o.UnProvisionedFromWaitNonLeader()
	}
}

func (o *smon) UnProvisionedFromIdle() {
	if o.UnProvisionedClearIfReached() {
		return
	}
	if o.isUnprovisionLeader() {
		o.transitionTo(statusWaitNonLeader)
		return
	} else {
		o.doAction(o.crmUnprovisionNonLeader, statusUnProvisioning, statusIdle, statusUnProvisionFailed)
	}
}

func (o *smon) UnProvisionedFromWaitNonLeader() {
	if o.UnProvisionedClearIfReached() {
		return
	}
	if !o.isUnprovisionLeader() {
		o.transitionTo(statusIdle)
		return
	}
	if o.hasNonLeaderProvisioned() {
		return
	}
	o.doAction(o.crmUnprovisionLeader, statusUnProvisioning, statusIdle, statusUnProvisionFailed)
}

func (o *smon) hasNonLeaderProvisioned() bool {
	for _, node := range o.scopeNodes {
		if node == o.localhost {
			continue
		}
		if otherInstStatus, ok := o.instStatus[node]; ok {
			if otherInstStatus.Provisioned.Bool() {
				return true
			}
		}
	}
	return false
}

func (o *smon) UnProvisionedClearIfReached() bool {
	if !o.instStatus[o.localhost].Provisioned.Bool() {
		//o.log.Info().Msg("global expect unprovisioned local status is not provisioned, unset global expect")
		o.log.Info().Msg(o.logMsg("local status is not provisioned, unset global expect"))
		o.change = true
		o.state.GlobalExpect = globalExpectUnset
		if o.state.LocalExpect != statusIdle {
			o.state.LocalExpect = statusIdle
		}
		return true
	}
	return false
}

func (o *smon) isUnprovisionLeader() bool {
	return o.isProvisioningLeader()
}
