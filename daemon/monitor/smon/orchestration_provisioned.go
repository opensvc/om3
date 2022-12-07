package smon

import (
	"sort"

	"opensvc.com/opensvc/core/provisioned"
)

func (o *smon) orchestrateProvisioned() {
	switch o.state.Status {
	case statusIdle:
		o.provisionedFromIdle()
	case statusWaitLeader:
		o.provisionedFromWaitLeader()
	case statusProvisionFailed:
		o.provisionedFromProvisionFailed()
	}
}

func (o *smon) provisionedFromProvisionFailed() {
	if o.provisionedClearIfReached() {
		return
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
	if o.instStatus[o.localhost].Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable) {
		o.log.Info().Msg("provisioned orchestration: local status provisioned, unset global expect")
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

func (o *smon) leaders() []string {
	l := make([]string, 0)
	for node, instSmon := range o.instSmon {
		if instSmon.IsLeader {
			l = append(l, node)
		}
	}
	if o.state.IsLeader {
		l = append(l, o.localhost)
	}
	return l
}

// provisioningLeader returns one of all leaders.
// Select the first in alphalexical order.
func (o *smon) provisioningLeader() string {
	leaders := o.leaders()
	switch len(leaders) {
	case 0:
		return ""
	case 1:
		return leaders[0]
	default:
		sort.StringSlice(leaders).Sort()
		return leaders[0]
	}
}

func (o *smon) isProvisioningLeader() bool {
	if o.provisioningLeader() == o.localhost {
		return true
	}
	return false
}

func (o *smon) hasLeaderProvisioned() bool {
	leader := o.provisioningLeader()
	if leaderInstanceStatus, ok := o.instStatus[leader]; !ok {
		return false
	} else if leaderInstanceStatus.Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable) {
		return true
	}
	return false
}
