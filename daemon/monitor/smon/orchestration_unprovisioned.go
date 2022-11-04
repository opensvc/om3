package smon

import (
	"opensvc.com/opensvc/core/provisioned"
)

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
		if o.hasNonLeaderProvisioned() {
			o.transitionTo(statusWaitNonLeader)
		} else {
			o.doAction(o.crmUnprovisionLeader, statusUnProvisioning, statusIdle, statusUnProvisionFailed)
		}
	} else {
		// immediate action on non-leaders
		o.doAction(o.crmUnprovisionNonLeader, statusUnProvisioning, statusIdle, statusUnProvisionFailed)
	}
}

func (o *smon) UnProvisionedFromWaitNonLeader() {
	if o.UnProvisionedClearIfReached() {
		o.transitionTo(statusIdle)
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
	for node, otherInstStatus := range o.instStatus {
		var isLeader bool
		if node == o.localhost {
			isLeader = o.state.IsLeader
		} else if instSmon, ok := o.instSmon[node]; ok {
			isLeader = instSmon.IsLeader
		}
		if isLeader {
			continue
		}
		if otherInstStatus.Provisioned.IsOneOf(provisioned.True, provisioned.Mixed) {
			return true
		}
	}
	return false
}

func (o *smon) UnProvisionedClearIfReached() bool {
	if o.instStatus[o.localhost].Provisioned.IsOneOf(provisioned.False, provisioned.NotApplicable) {
		o.loggerWithState().Info().Msg("local status is not provisioned, unset global expect")
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
