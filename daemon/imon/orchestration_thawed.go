package imon

import (
	"github.com/opensvc/om3/core/instance"
)

func (t *Manager) orchestrateThawed() {
	switch t.state.State {
	case instance.MonitorStateIdle,
		instance.MonitorStateStartFailure,
		instance.MonitorStateStartSuccess,
		instance.MonitorStateStopFailure,
		instance.MonitorStateStopSuccess,
		instance.MonitorStatePurgeFailed,
		instance.MonitorStateProvisionFailure,
		instance.MonitorStateProvisionSuccess,
		instance.MonitorStateUnprovisionFailure,
		instance.MonitorStateUnprovisionSuccess,
		instance.MonitorStateReady:
		t.ThawedFromIdle()
	}
}

func (t *Manager) ThawedFromIdle() {
	if t.thawedClearIfReached() {
		return
	}
	t.doTransitionAction(t.unfreeze, instance.MonitorStateThawProgress, instance.MonitorStateIdle, instance.MonitorStateThawFailure)
}

func (t *Manager) thawedClearIfReached() bool {
	if t.instStatus[t.localhost].IsThawed() {
		t.log.Infof("instance state is thawed -> set reached")
		t.doneAndIdle()
		t.clearPending()
		return true
	}
	return false
}
