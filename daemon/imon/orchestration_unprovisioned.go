package imon

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
)

func (t *Manager) orchestrateUnprovisioned() {
	t.disableMonitor("orchestrate unprovisioned")
	switch t.state.State {
	case instance.MonitorStateIdle,
		instance.MonitorStateProvisionFailure,
		instance.MonitorStateStartFailure:
		t.UnprovisionedFromIdle()
	case instance.MonitorStateWaitNonLeader:
		t.UnprovisionedFromWaitNonLeader()
	case instance.MonitorStateWaitChildren:
		t.setWaitChildren()
	case instance.MonitorStateUnprovisionSuccess,
		instance.MonitorStateUnprovisionFailure:
		if t.unprovisionedClearIfReached() {
			return
		}
	}
}

func (t *Manager) UnprovisionedFromIdle() {
	if t.unprovisionedClearIfReached() {
		return
	}
	if t.setWaitChildren() {
		return
	}
	if t.isUnprovisionLeader() {
		if t.hasNonLeaderProvisioned() {
			t.transitionTo(instance.MonitorStateWaitNonLeader)
		} else {
			t.queueAction(t.crmUnprovisionLeader, instance.MonitorStateUnprovisionProgress, instance.MonitorStateUnprovisionSuccess, instance.MonitorStateUnprovisionFailure)
		}
	} else {
		// immediate action on non-leaders
		t.queueAction(t.crmUnprovisionNonLeader, instance.MonitorStateUnprovisionProgress, instance.MonitorStateUnprovisionSuccess, instance.MonitorStateUnprovisionFailure)
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
	if t.hasNonLeaderWithState(instance.MonitorStateUnprovisionFailure) {
		t.transitionTo(instance.MonitorStateUnprovisionFailure)
		return
	}
	if t.hasNonLeaderProvisioned() {
		return
	}
	t.queueAction(t.crmUnprovisionLeader, instance.MonitorStateUnprovisionProgress, instance.MonitorStateUnprovisionSuccess, instance.MonitorStateUnprovisionFailure)
}

func (t *Manager) hasNonLeaderWithState(states ...instance.MonitorState) bool {
	for node, instMon := range t.instMonitor {
		var isLeader bool
		if node == t.localhost {
			isLeader = t.state.IsLeader
		} else {
			isLeader = instMon.IsLeader
		}
		if isLeader {
			continue
		}
		if instMon.State.IsOneOf(states...) {
			return true
		}
	}
	return false
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
	reached := func(msg string, toIdle bool) bool {
		t.log.Infof("%s -> set reached", msg)
		if toIdle {
			t.doneAndIdle()
		} else {
			t.done()
		}
		t.disableMonitor(msg)
		t.updateIfChange()
		return true
	}
	if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.False, provisioned.NotApplicable) {
		return reached("unprovisioned orchestration: instance is not provisioned", true)
	}
	if t.instStatus[t.localhost].Avail == status.NotApplicable {
		return reached("unprovisioned orchestration: instance availability is n/a", true)
	}
	if t.isAllState(instance.MonitorStateUnprovisionFailure) {
		return reached("unprovisioned orchestration: all instances unprovision failed", false)
	}
	return false
}

func (t *Manager) isUnprovisionLeader() bool {
	return t.isProvisioningLeader()
}
