package imon

import (
	"github.com/opensvc/om3/v3/core/instance"
)

func (t *Manager) orchestrateDeleted() {
	t.log.Tracef("orchestrateDeleted starting from %s", t.state.State)
	switch t.state.State {
	case instance.MonitorStateDeleteSuccess:
		t.deletedFromDeleted()
	case instance.MonitorStateIdle,
		instance.MonitorStateBootFailed,
		instance.MonitorStateFreezeFailure,
		instance.MonitorStateProvisionFailure,
		instance.MonitorStateStartFailure,
		instance.MonitorStateStopFailure,
		instance.MonitorStateUnfreezeFailure,
		instance.MonitorStateUnprovisionFailure:
		t.deletedFromIdle()
	case instance.MonitorStateWaitChildren:
		t.deletedFromWaitChildren()
	case instance.MonitorStateDeleteProgress:
	default:
		t.log.Warnf("orchestrateDeleted has no solution from state %s", t.state.State)
	}
}

func (t *Manager) deletedFromIdle() {
	if t.setWaitChildren() {
		return
	}
	t.queueAction(t.crmDelete, instance.MonitorStateDeleteProgress, instance.MonitorStateDeleteSuccess, instance.MonitorStateDeleteFailure)
	return
}

func (t *Manager) deletedFromDeleted() {
	t.log.Warnf("have been deleted, we should die soon")
}

func (t *Manager) deletedFromWaitChildren() {
	if t.setWaitChildren() {
		return
	}
	t.queueAction(t.crmDelete, instance.MonitorStateDeleteProgress, instance.MonitorStateDeleteSuccess, instance.MonitorStateDeleteFailure)
	return
}
