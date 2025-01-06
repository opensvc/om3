package imon

import (
	"sort"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/topology"
)

func (t *Manager) orchestrateProvisioned() {
	switch t.state.State {
	case instance.MonitorStateIdle,
		instance.MonitorStateStopFailed,
		instance.MonitorStateThawed,
		instance.MonitorStateUnprovisionFailed:
		t.provisionedFromIdle()
	case instance.MonitorStateProvisioned:
		t.provisionedFromProvisioned()
	case instance.MonitorStateWaitLeader:
		t.provisionedFromWaitLeader()
	case instance.MonitorStateProvisionFailed:
		t.provisionedFromProvisionFailed()
	case instance.MonitorStateThawing:
	case instance.MonitorStateThawedFailed:
		// TODO: clear ?
	}
}

func (t *Manager) provisionedFromProvisioned() {
	t.doTransitionAction(t.unfreeze, instance.MonitorStateThawing, instance.MonitorStateThawed, instance.MonitorStateThawedFailed)
}

func (t *Manager) provisionedFromProvisionFailed() {
	if t.provisionedClearIfReached() {
		return
	}
}

func (t *Manager) provisionedFromIdle() {
	if t.provisionedClearIfReached() {
		return
	}
	if t.isProvisioningLeader() {
		t.queueAction(t.crmProvisionLeader, instance.MonitorStateProvisioning, instance.MonitorStateProvisioned, instance.MonitorStateProvisionFailed)
		return
	} else {
		t.transitionTo(instance.MonitorStateWaitLeader)
	}
}

func (t *Manager) provisionedFromWaitLeader() {
	if t.provisionedClearIfReached() {
		t.transitionTo(instance.MonitorStateIdle)
		return
	}
	if !t.hasLeaderProvisioned() {
		return
	}
	t.queueAction(t.crmProvisionNonLeader, instance.MonitorStateProvisioning, instance.MonitorStateProvisioned, instance.MonitorStateProvisionFailed)
	return
}

func (t *Manager) provisionedClearIfReached() bool {
	reached := func(msg string) bool {
		if t.instStatus[t.localhost].IsFrozen() {
			t.doUnfreeze()
		}
		t.log.Infof(msg)
		t.doneAndIdle()
		if t.isLocalStarted() {
			t.enableMonitor("instance is now started")
		}
		t.updateIfChange()
		return true
	}
	if t.isAllState(instance.MonitorStateProvisionFailed) {
		t.loggerWithState().Infof("all instances provision failed -> set done")
		t.done()
		return true
	}
	if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable) {
		return reached("provisioned orchestration: instance is provisioned -> set reached")
	}
	if t.instStatus[t.localhost].Avail == status.NotApplicable {
		return reached("provisioned orchestration: instance availability is n/a -> set reached, clear local expect")
	}
	return false
}

func (t *Manager) leaders() []string {
	l := make([]string, 0)
	for node, instMon := range t.instMonitor {
		if instMon.IsLeader {
			l = append(l, node)
		}
	}
	if t.state.IsLeader {
		l = append(l, t.localhost)
	}
	return l
}

// provisioningLeader returns one of all leaders.
// Select the first in alphalexical order.
func (t *Manager) provisioningLeader() string {
	leaders := t.leaders()
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

func (t *Manager) isProvisioningLeader() bool {
	if t.objStatus.Topology == topology.Flex {
		return t.state.IsLeader
	} else {
		if t.provisioningLeader() == t.localhost {
			return true
		}
		return false
	}
}

func (t *Manager) hasLeaderProvisioned() bool {
	leader := t.provisioningLeader()
	if leaderInstanceStatus, ok := t.instStatus[leader]; !ok {
		return false
	} else if leaderInstanceStatus.Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable) {
		return true
	}
	return false
}
