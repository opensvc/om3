package smon

import (
	"opensvc.com/opensvc/core/provisioned"
)

func (o *smon) orchestrateUnprovisioned() {
	switch o.state.Status {
	case statusIdle:
		o.UnprovisionedFromIdle()
	case statusWaitNonLeader:
		o.UnprovisionedFromWaitNonLeader()
	}
}

func (o *smon) UnprovisionedFromIdle() {
	if o.UnprovisionedClearIfReached() {
		return
	}
	if o.isUnprovisionLeader() {
		if o.hasNonLeaderProvisioned() {
			o.transitionTo(statusWaitNonLeader)
		} else {
			o.doAction(o.crmUnprovisionLeader, statusUnprovisioning, statusIdle, statusUnprovisionFailed)
		}
	} else {
		// immediate action on non-leaders
		o.doAction(o.crmUnprovisionNonLeader, statusUnprovisioning, statusIdle, statusUnprovisionFailed)
	}
}

func (o *smon) UnprovisionedFromWaitNonLeader() {
	if o.UnprovisionedClearIfReached() {
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
	o.doAction(o.crmUnprovisionLeader, statusUnprovisioning, statusIdle, statusUnprovisionFailed)
}

func (o *smon) hasNonLeaderProvisioned() bool {
	for node, otherInstStatus := range o.instStatus {
		var isLeader bool
		if node == o.localhost {
			isLeader = o.state.IsLeader
		} else if instMon, ok := o.instMonitor[node]; ok {
			isLeader = instMon.IsLeader
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

func (o *smon) UnprovisionedClearIfReached() bool {
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
