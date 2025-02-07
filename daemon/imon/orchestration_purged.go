package imon

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
)

func (t *Manager) orchestratePurged() {
	t.log.Debugf("orchestratePurged starting from %s", t.state.State)
	switch t.state.State {
	case instance.MonitorStateDeleteSuccess:
		t.purgedFromDeleted()
	case instance.MonitorStateIdle:
		t.purgedFromIdle()
	case instance.MonitorStateStopSuccess:
		t.purgedFromStopped()
	case instance.MonitorStateStopFailure:
		t.done()
	case instance.MonitorStateUnprovisionSuccess:
		t.purgedFromUnprovisioned()
	case instance.MonitorStateWaitNonLeader:
		t.purgedFromWaitNonLeader()
	case instance.MonitorStatePurgeFailed:
		t.done()
	case instance.MonitorStateUnprovisionProgress,
		instance.MonitorStateDeleteProgress,
		instance.MonitorStateRunning,
		instance.MonitorStateStopProgress:
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
	go t.orchestrateAfterAction(instance.MonitorStateIdle, instance.MonitorStateUnprovisionSuccess)
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
	go t.orchestrateAfterAction(instance.MonitorStateStopSuccess, instance.MonitorStateUnprovisionSuccess)
	return
}

func (t *Manager) purgedFromDeleted() {
	t.change = true
	t.state.GlobalExpect = instance.MonitorGlobalExpectNone
	t.state.State = instance.MonitorStateIdle
	t.updateIfChange()
}

func (t *Manager) purgedFromUnprovisioned() {
	t.queueAction(t.crmDelete, instance.MonitorStateDeleteProgress, instance.MonitorStateDeleteSuccess, instance.MonitorStatePurgeFailed)
}

func (t *Manager) purgedFromIdleUp() {
	t.disableMonitor("orchestrate purged stopping")
	t.queueAction(t.crmStop, instance.MonitorStateStopProgress, instance.MonitorStateStopSuccess, instance.MonitorStateStopFailure)
}

func (t *Manager) purgedFromIdleProvisioned() {
	if t.isUnprovisionLeader() {
		t.transitionTo(instance.MonitorStateWaitNonLeader)
		t.purgedFromWaitNonLeader()
		return
	}
	t.queueAction(t.crmUnprovisionNonLeader, instance.MonitorStateUnprovisionProgress, instance.MonitorStateUnprovisionSuccess, instance.MonitorStatePurgeFailed)
}

func (t *Manager) purgedFromWaitNonLeader() {
	if !t.isUnprovisionLeader() {
		t.transitionTo(instance.MonitorStateIdle)
		return
	}
	if t.hasNonLeaderProvisioned() {
		return
	}
	t.queueAction(t.crmUnprovisionLeader, instance.MonitorStateUnprovisionProgress, instance.MonitorStateUnprovisionSuccess, instance.MonitorStatePurgeFailed)
}
