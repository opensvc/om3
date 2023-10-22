package imon

import (
	"sort"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
)

func (o *imon) orchestrateProvisioned() {
	switch o.state.State {
	case instance.MonitorStateIdle,
		instance.MonitorStateStopFailed,
		instance.MonitorStateUnprovisionFailed:
		o.provisionedFromIdle()
	case instance.MonitorStateWaitLeader:
		o.provisionedFromWaitLeader()
	case instance.MonitorStateProvisionFailed:
		o.provisionedFromProvisionFailed()
	}
}

func (o *imon) provisionedFromProvisionFailed() {
	if o.provisionedClearIfReached() {
		return
	}
}

func (o *imon) provisionedFromIdle() {
	if o.provisionedClearIfReached() {
		return
	}
	if o.isProvisioningLeader() {
		o.doAction(o.crmProvisionLeader, instance.MonitorStateProvisioning, instance.MonitorStateIdle, instance.MonitorStateProvisionFailed)
		return
	} else {
		o.transitionTo(instance.MonitorStateWaitLeader)
	}
}

func (o *imon) provisionedFromWaitLeader() {
	if o.provisionedClearIfReached() {
		o.transitionTo(instance.MonitorStateIdle)
		return
	}
	if !o.hasLeaderProvisioned() {
		return
	}
	o.doAction(o.crmProvisionNonLeader, instance.MonitorStateProvisioning, instance.MonitorStateIdle, instance.MonitorStateProvisionFailed)
	return
}

func (o *imon) provisionedClearIfReached() bool {
	reached := func(msg string) bool {
		o.log.Info().Msgf("daemon: imon: %s: "+msg, o.path)
		o.doneAndIdle()
		o.state.LocalExpect = instance.MonitorLocalExpectNone
		o.updateIfChange()
		return true
	}
	if o.instStatus[o.localhost].Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable) {
		return reached("provisioned orchestration: instance is provisioned -> set reached, clear local expect")
	}
	if o.instStatus[o.localhost].Avail == status.NotApplicable {
		return reached("provisioned orchestration: instance availability is n/a -> set reached, clear local expect")
	}
	return false
}

func (o *imon) leaders() []string {
	l := make([]string, 0)
	for node, instMon := range o.instMonitor {
		if instMon.IsLeader {
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
func (o *imon) provisioningLeader() string {
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

func (o *imon) isProvisioningLeader() bool {
	if o.provisioningLeader() == o.localhost {
		return true
	}
	return false
}

func (o *imon) hasLeaderProvisioned() bool {
	leader := o.provisioningLeader()
	if leaderInstanceStatus, ok := o.instStatus[leader]; !ok {
		return false
	} else if leaderInstanceStatus.Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable) {
		return true
	}
	return false
}
