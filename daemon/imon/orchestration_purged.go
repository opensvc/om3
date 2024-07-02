package imon

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
)

func (t *Manager) orchestratePurged() {
	t.log.Debugf("orchestratePurged starting from %s", t.state.State)
	switch t.state.State {
	case instance.MonitorStateDeleted:
		t.purgedFromDeleted()
	case instance.MonitorStateIdle:
		t.purgedFromIdle()
	case instance.MonitorStateStopped:
		t.purgedFromStopped()
	case instance.MonitorStateStopFailed:
		t.done()
	case instance.MonitorStateUnprovisioned:
		t.purgedFromUnprovisioned()
	case instance.MonitorStateWaitNonLeader:
		t.purgedFromWaitNonLeader()
	case instance.MonitorStateUnprovisioning,
		instance.MonitorStateDeleting,
		instance.MonitorStateRunning,
		instance.MonitorStateStopping:
	default:
		t.log.Warnf("orchestratePurged has no solution from state %s", t.state.State)
	}
}

func (t *Manager) purgedFromIdle() {
	if t.instStatus[t.localhost].Avail == status.Up {
		t.purgedFromIdleUp()
		return
	}
	if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable) {
		t.purgedFromIdleProvisioned()
		return
	}
	go t.orchestrateAfterAction(instance.MonitorStateIdle, instance.MonitorStateUnprovisioned)
	return
}

func (t *Manager) purgedFromStopped() {
	if t.instStatus[t.localhost].Avail.Is(status.Up, status.Warn) {
		t.log.Debugf("purgedFromStopped return on o.instStatus[o.localhost].Avail.Is(status.Up, status.Warn)")
		return
	}
	if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable) {
		t.log.Debugf("purgedFromStopped return on o.instStatus[o.localhost].Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable)")
		t.purgedFromIdleProvisioned()
		return
	}
	go t.orchestrateAfterAction(instance.MonitorStateStopped, instance.MonitorStateUnprovisioned)
	return
}

func (t *Manager) purgedFromDeleted() {
	t.change = true
	t.state.GlobalExpect = instance.MonitorGlobalExpectNone
	t.state.State = instance.MonitorStateIdle
	t.updateIfChange()
}

func (t *Manager) purgedFromUnprovisioned() {
	t.queueAction(t.crmDelete, instance.MonitorStateDeleting, instance.MonitorStateDeleted, instance.MonitorStatePurgeFailed)
}

func (t *Manager) purgedFromIdleUp() {
	t.queueAction(t.crmStop, instance.MonitorStateStopping, instance.MonitorStateStopped, instance.MonitorStateStopFailed)
}

func (t *Manager) purgedFromIdleProvisioned() {
	if t.isUnprovisionLeader() {
		t.transitionTo(instance.MonitorStateWaitNonLeader)
		t.purgedFromWaitNonLeader()
		return
	}
	t.queueAction(t.crmUnprovisionNonLeader, instance.MonitorStateUnprovisioning, instance.MonitorStateUnprovisioned, instance.MonitorStatePurgeFailed)
}

func (t *Manager) purgedFromWaitNonLeader() {
	if !t.isUnprovisionLeader() {
		t.transitionTo(instance.MonitorStateIdle)
		return
	}
	if t.hasNonLeaderProvisioned() {
		return
	}
	t.queueAction(t.crmUnprovisionLeader, instance.MonitorStateUnprovisioning, instance.MonitorStateUnprovisioned, instance.MonitorStatePurgeFailed)
}
