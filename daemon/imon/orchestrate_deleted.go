package imon

import (
	"github.com/opensvc/om3/core/instance"
)

func (t *Manager) orchestrateDeleted() {
	t.log.Debugf("orchestrateDeleted starting from %s", t.state.State)
	switch t.state.State {
	case instance.MonitorStateDeleted:
		t.deletedFromDeleted()
	case instance.MonitorStateIdle,
		instance.MonitorStateBootFailed,
		instance.MonitorStateFreezeFailed,
		instance.MonitorStateProvisionFailed,
		instance.MonitorStateStartFailed,
		instance.MonitorStateStopFailed,
		instance.MonitorStateThawedFailed,
		instance.MonitorStateUnprovisionFailed:
		t.deletedFromIdle()
	case instance.MonitorStateWaitChildren:
		t.deletedFromWaitChildren()
	case instance.MonitorStateDeleting:
	default:
		t.log.Warnf("orchestrateDeleted has no solution from state %s", t.state.State)
	}
}

func (t *Manager) deletedFromIdle() {
	if t.setWaitChildren() {
		return
	}
	t.queueAction(t.crmDelete, instance.MonitorStateDeleting, instance.MonitorStateDeleted, instance.MonitorStateDeleteFailed)
	return
}

func (t *Manager) deletedFromDeleted() {
	t.log.Warnf("have been deleted, we should die soon")
}

func (t *Manager) deletedFromWaitChildren() {
	if t.setWaitChildren() {
		return
	}
	t.queueAction(t.crmDelete, instance.MonitorStateDeleting, instance.MonitorStateDeleted, instance.MonitorStateDeleteFailed)
	return
}
