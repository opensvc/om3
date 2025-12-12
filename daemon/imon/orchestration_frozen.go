package imon

import "github.com/opensvc/om3/v3/core/instance"

func (t *Manager) orchestrateFrozen() {
	switch t.state.State {
	case instance.MonitorStateIdle,
		instance.MonitorStateStartFailure,
		instance.MonitorStateStopFailure,
		instance.MonitorStatePurgeFailed,
		instance.MonitorStateProvisionFailure,
		instance.MonitorStateUnprovisionFailure,
		instance.MonitorStateReady:
		t.frozenFromIdle()
	default:
		t.log.Warnf("orchestrateFrozen has no solution from state %s", t.state.State)
	}
}

func (t *Manager) frozenFromIdle() {
	if t.frozenClearIfReached() {
		return
	}
	t.doTransitionAction(t.freeze, instance.MonitorStateFreezeProgress, instance.MonitorStateIdle, instance.MonitorStateFreezeFailure)
}

func (t *Manager) frozenClearIfReached() bool {
	if t.instStatus[t.localhost].IsFrozen() {
		t.log.Infof("instance state is frozen -> set reached")
		t.doneAndIdle()
		t.clearPending()
		return true
	}
	return false
}
