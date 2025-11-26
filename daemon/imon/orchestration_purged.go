package imon

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/provisioned"
)

var (
	kindWithImmediatePurge = naming.NewKinds(naming.KindSvc, naming.KindVol)
)

func (t *Manager) orchestratePurged() {
	t.log.Tracef("orchestratePurged starting from %s", t.state.State)
	switch t.state.State {
	case instance.MonitorStateDeleteSuccess:
		t.purgedFromDeleted()
	case instance.MonitorStateIdle:
		t.purgedFromIdle()
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
		instance.MonitorStateRunning:
	case instance.MonitorStateWaitChildren:
		t.setWaitChildren()
	default:
		t.log.Warnf("orchestratePurged has no solution from state %s", t.state.State)
	}
}

func (t *Manager) purgedFromIdle() {
	if !kindWithImmediatePurge.Has(t.path.Kind) {
		t.transitionTo(instance.MonitorStateUnprovisionSuccess)
		return
	}
	if t.setWaitChildren() {
		return
	}
	if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable) {
		t.purgedFromIdleProvisioned()
		return
	}
	go t.orchestrateAfterAction(instance.MonitorStateIdle, instance.MonitorStateUnprovisionSuccess)
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
