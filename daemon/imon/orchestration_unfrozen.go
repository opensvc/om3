package imon

import (
	"github.com/opensvc/om3/v3/core/instance"
)

func (t *Manager) orchestrateUnfrozen() {
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
		t.UnfrozenFromIdle()
	}
}

func (t *Manager) UnfrozenFromIdle() {
	if t.unfrozenClearIfReached() {
		return
	}
	t.doTransitionAction(t.unfreeze, instance.MonitorStateUnfreezeProgress, instance.MonitorStateIdle, instance.MonitorStateUnfreezeFailure)
}

func (t *Manager) unfrozenClearIfReached() bool {
	if t.instStatus[t.localhost].IsUnfrozen() {
		t.log.Infof("instance state is unfrozen: expectation reached")
		t.doneAndIdle()
		t.clearPending()
		return true
	}
	return false
}
