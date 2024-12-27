package imon

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
)

func (t *Manager) orchestrateUnprovisioned() {
	t.disableLocalExpect("orchestrate unprovisioned")
	switch t.state.State {
	case instance.MonitorStateIdle,
		instance.MonitorStateProvisionFailed,
		instance.MonitorStateStartFailed:
		t.UnprovisionedFromIdle()
	case instance.MonitorStateWaitNonLeader:
		t.UnprovisionedFromWaitNonLeader()
	}
}

func (t *Manager) UnprovisionedFromIdle() {
	if t.unprovisionedClearIfReached() {
		return
	}
	if t.isUnprovisionLeader() {
		if t.hasNonLeaderProvisioned() {
			t.transitionTo(instance.MonitorStateWaitNonLeader)
		} else {
			t.queueLastAction(t.crmUnprovisionLeader, instance.MonitorStateUnprovisioning, instance.MonitorStateIdle, instance.MonitorStateUnprovisionFailed)
		}
	} else {
		// immediate action on non-leaders
		t.queueLastAction(t.crmUnprovisionNonLeader, instance.MonitorStateUnprovisioning, instance.MonitorStateIdle, instance.MonitorStateUnprovisionFailed)
	}
}

func (t *Manager) UnprovisionedFromWaitNonLeader() {
	if t.unprovisionedClearIfReached() {
		t.transitionTo(instance.MonitorStateIdle)
		return
	}
	if !t.isUnprovisionLeader() {
		t.transitionTo(instance.MonitorStateIdle)
		return
	}
	if t.hasNonLeaderProvisioned() {
		return
	}
	t.queueLastAction(t.crmUnprovisionLeader, instance.MonitorStateUnprovisioning, instance.MonitorStateIdle, instance.MonitorStateUnprovisionFailed)
}

func (t *Manager) hasNonLeaderProvisioned() bool {
	for node, otherInstStatus := range t.instStatus {
		var isLeader bool
		if node == t.localhost {
			isLeader = t.state.IsLeader
		} else if instMon, ok := t.instMonitor[node]; ok {
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

func (t *Manager) unprovisionedClearIfReached() bool {
	reached := func(msg string) bool {
		t.log.Infof("%s -> set reached", msg)
		t.doneAndIdle()
		t.disableLocalExpect(msg)
		t.updateIfChange()
		return true
	}
	if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.False, provisioned.NotApplicable) {
		return reached("unprovisioned orchestration: instance is not provisioned")
	}
	if t.instStatus[t.localhost].Avail == status.NotApplicable {
		return reached("unprovisioned orchestration: instance availability is n/a")
	}
	return false
}

func (t *Manager) isUnprovisionLeader() bool {
	return t.isProvisioningLeader()
}
