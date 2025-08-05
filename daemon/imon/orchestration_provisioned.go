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
		instance.MonitorStateStopFailure,
		instance.MonitorStateUnfreezeSuccess,
		instance.MonitorStateUnprovisionFailure:
		t.provisionedFromIdle()
	case instance.MonitorStateStartSuccess:
		t.provisionedFromProvisioned()
	case instance.MonitorStateProvisionSuccess:
		t.provisionedFromProvisioned()
	case instance.MonitorStateWaitLeader:
		t.provisionedFromWaitLeader()
	case instance.MonitorStateProvisionFailure:
		t.provisionedFromProvisionFailed()
	case instance.MonitorStateStartFailure:
		t.provisionedFromProvisionFailed()
	case instance.MonitorStateUnfreezeProgress:
	case instance.MonitorStateUnfreezeFailure:
		// TODO: clear ?
	}
}

func (t *Manager) provisionedFromProvisioned() {
	if t.instStatus[t.localhost].IsFrozen() {
		t.doTransitionAction(t.unfreeze, instance.MonitorStateUnfreezeProgress, instance.MonitorStateUnfreezeSuccess, instance.MonitorStateUnfreezeFailure)
	} else {
		t.doneAndIdle()
	}
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
		if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.NotApplicable, provisioned.True) {
			t.queueAction(t.crmStart, instance.MonitorStateStartProgress, instance.MonitorStateStartSuccess, instance.MonitorStateStartFailure)
		} else {
			t.queueAction(t.crmProvisionLeader, instance.MonitorStateProvisionProgress, instance.MonitorStateProvisionSuccess, instance.MonitorStateProvisionFailure)
		}
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
	if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.NotApplicable, provisioned.True) {
		t.queueAction(t.crmStartStandby, instance.MonitorStateStartProgress, instance.MonitorStateStartSuccess, instance.MonitorStateStartFailure)
	} else {
		t.queueAction(t.crmProvisionNonLeader, instance.MonitorStateProvisionProgress, instance.MonitorStateProvisionSuccess, instance.MonitorStateProvisionFailure)
	}
	return
}

func (t *Manager) provisionedClearIfReached() bool {
	reached := func(msg string, succeed bool) bool {
		if succeed && t.instStatus[t.localhost].IsFrozen() {
			t.doUnfreeze()
		}
		if succeed {
			t.log.Infof("provisioned orchestration reached: %s", msg)
			t.doneAndIdle()
		} else {
			t.log.Infof("provisioned orchestration done: %s", msg)
			t.done()
		}
		if succeed && t.isLocalStarted() {
			t.enableMonitor("instance is now started")
		}
		t.updateIfChange()
		return true
	}

	// failures
	if t.isAllState(instance.MonitorStateProvisionFailure) {
		return reached("all instances provision failed", false)
	} else if t.hasLeaderProvisionedFailed() {
		return reached("leader instance is provision failed", false)
	} else if t.state.State.IsOneOf(instance.MonitorStateProvisionFailure) {
		return reached("instance is provision failed", false)
	}

	if !t.isStarted() && !t.objStatus.Avail.Is(status.NotApplicable) {
		return false
	}

	// succeeds
	if t.instStatus[t.localhost].Provisioned == provisioned.True {
		return reached("instance is provisioned", true)
	} else if t.instStatus[t.localhost].Avail == status.NotApplicable {
		return reached("instance availability is n/a", true)
	} else if t.instStatus[t.localhost].Provisioned == provisioned.NotApplicable {
		if t.isProvisioningLeader() && t.instStatus[t.localhost].Avail.Is(status.Up) {
			return reached("unprovisionable leader instance is up", true)
		} else if !t.isProvisioningLeader() && t.instStatus[t.localhost].Avail.Is(status.Down, status.StandbyUp, status.StandbyUpWithDown, status.StandbyUpWithUp) {
			return reached("unprovisionable leader instance is up", true)
		}
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

func (t *Manager) hasLeaderProvisionedFailed() bool {
	leader := t.provisioningLeader()
	if t.instMonitor[leader].State.IsOneOf(instance.MonitorStateProvisionFailure) {
		return true
	}
	return false
}
