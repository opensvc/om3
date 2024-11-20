package imon

import (
	"github.com/opensvc/om3/core/instance"
)

func (t *Manager) orchestrateThawed() {
	switch t.state.State {
	case instance.MonitorStateIdle,
		instance.MonitorStateStartFailed,
		instance.MonitorStateStarted,
		instance.MonitorStateStopFailed,
		instance.MonitorStateStopped,
		instance.MonitorStatePurgeFailed,
		instance.MonitorStateProvisionFailed,
		instance.MonitorStateProvisioned,
		instance.MonitorStateUnprovisionFailed,
		instance.MonitorStateUnprovisioned,
		instance.MonitorStateReady:
		t.ThawedFromIdle()
	}
}

func (t *Manager) ThawedFromIdle() {
	if t.thawedClearIfReached() {
		return
	}
	t.doTransitionAction(t.unfreeze, instance.MonitorStateThawing, instance.MonitorStateIdle, instance.MonitorStateThawedFailed)
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
