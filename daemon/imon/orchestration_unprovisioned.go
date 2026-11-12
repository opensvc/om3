package imon

import (
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
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
	case instance.MonitorStateUnprovisionSuccess:
		if !t.unprovisionedClearIfReached() {
			t.unprovisionedFromUnprovisionSuccess()
		}
	case instance.MonitorStateUnprovisionFailure:
		t.unprovisionedFromUnprovisionFailed()
	}
}

// unprovisionedFromUnprovisionFailed ends the orchestration on the local
// instance when its unprovision failed and left it provisioned: a failed
// action is not retried, so nothing is left to run it, and the instances
// done with theirs would otherwise wait for this one forever. The failure
// lingers, so an operator can see what failed, as a purge failure does.
//
// An unprovision that failed and left the instance unprovisioned all the
// same has reached the state asked for, which the reached check says first.
func (t *Manager) unprovisionedFromUnprovisionFailed() {
	if t.unprovisionedClearIfReached() {
		return
	}
	if t.state.OrchestrationIsDone {
		return
	}
	t.loggerWithState().Infof("local instance unprovision failed -> set done")
	t.done()
}

// unprovisionedFromUnprovisionSuccess ends the orchestration as an
// unprovision failure when the unprovision succeeded and a status taken after
// it still reads the instance provisioned: a resource flagged provisioned
// that the unprovision removed, as a status evaluation running beside the
// unprovision wrote back, held the orchestration for ever, nothing left to
// run it again. The failure lingers, so an operator sees what is left.
//
// A status taken before the end of the unprovision says nothing of it, as the
// one the unprovision command posted, dated before the monitor changed state:
// a status refresh is asked, once, and its result decides.
func (t *Manager) unprovisionedFromUnprovisionSuccess() {
	if !t.isLocalStatusCurrent() {
		return
	}
	st := t.instStatus[t.localhost]
	if !st.Provisioned.IsOneOf(provisioned.True, provisioned.Mixed) {
		return
	}
	endedAt := t.state.StateUpdatedAt
	if !st.UpdatedAt.After(endedAt) {
		if !t.unprovisionStatusAskedFor.Equal(endedAt) {
			t.unprovisionStatusAskedFor = endedAt
			t.log.Infof("unprovisioned orchestration: the instance is %s, wait for a status taken after the unprovision", st.Provisioned)
			t.requestStatusRefresh(t.instConfig.Priority)
		}
		return
	}
	t.loggerWithState().Warnf("unprovisioned orchestration: the unprovision succeeded, but a status taken after it says the instance is %s -> unprovision failed", st.Provisioned)
	t.transitionTo(instance.MonitorStateUnprovisionFailure)
	t.unprovisionedFromUnprovisionFailed()
}

func (t *Manager) UnprovisionedFromIdle() {
	if t.unprovisionedClearIfReached() {
		return
	}
	if t.setWaitChildren() || !t.isLocalStatusCurrent() {
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
	if t.hasNonLeaderProvisioned() || !t.isLocalStatusCurrent() {
		return
	}
	t.queueAction(t.crmUnprovisionLeader, instance.MonitorStateUnprovisionProgress, instance.MonitorStateUnprovisionSuccess, instance.MonitorStateUnprovisionFailure)
}

func (t *Manager) hasNonLeaderWithState(states ...instance.MonitorState) bool {
	for node, instMon := range t.instMonitor {
		if t.isUnprovisionLeaderNode(node) {
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
		if t.isUnprovisionLeaderNode(node) {
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
	if !t.isLocalStatusCurrent() {
		return false
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

// isUnprovisionLeaderNode says the instance of node is the one unprovisioned
// last, by the rule isUnprovisionLeader applies to the local one, see
// isProvisioningLeaderNode. The non-leaders are told apart by the same rule,
// or the leader would count itself among them and wait for itself.
func (t *Manager) isUnprovisionLeaderNode(node string) bool {
	return t.isProvisioningLeaderNode(node)
}
