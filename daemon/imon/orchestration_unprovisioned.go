package imon

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
)

func (o *imon) orchestrateUnprovisioned() {
	switch o.state.State {
	case instance.MonitorStateIdle,
		instance.MonitorStateStartFailed:
		o.UnprovisionedFromIdle()
	case instance.MonitorStateWaitNonLeader:
		o.UnprovisionedFromWaitNonLeader()
	}
}

func (o *imon) UnprovisionedFromIdle() {
	if o.UnprovisionedClearIfReached() {
		return
	}
	if o.isUnprovisionLeader() {
		if o.hasNonLeaderProvisioned() {
			o.transitionTo(instance.MonitorStateWaitNonLeader)
		} else {
			o.doAction(o.crmUnprovisionLeader, instance.MonitorStateUnprovisioning, instance.MonitorStateIdle, instance.MonitorStateUnprovisionFailed)
		}
	} else {
		// immediate action on non-leaders
		o.doAction(o.crmUnprovisionNonLeader, instance.MonitorStateUnprovisioning, instance.MonitorStateIdle, instance.MonitorStateUnprovisionFailed)
	}
}

func (o *imon) UnprovisionedFromWaitNonLeader() {
	if o.UnprovisionedClearIfReached() {
		o.transitionTo(instance.MonitorStateIdle)
		return
	}
	if !o.isUnprovisionLeader() {
		o.transitionTo(instance.MonitorStateIdle)
		return
	}
	if o.hasNonLeaderProvisioned() {
		return
	}
	o.doAction(o.crmUnprovisionLeader, instance.MonitorStateUnprovisioning, instance.MonitorStateIdle, instance.MonitorStateUnprovisionFailed)
}

func (o *imon) hasNonLeaderProvisioned() bool {
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

func (o *imon) UnprovisionedClearIfReached() bool {
	if o.instStatus[o.localhost].Provisioned.IsOneOf(provisioned.False, provisioned.NotApplicable) {
		o.loggerWithState().Info().Msg("instance state is not provisioned -> set reached, clear local expect")
		o.setReached()
		o.state.LocalExpect = instance.MonitorLocalExpectNone
		return true
	}
	return false
}

func (o *imon) isUnprovisionLeader() bool {
	return o.isProvisioningLeader()
}
