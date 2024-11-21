package imon

import "github.com/opensvc/om3/core/instance"

func (t *Manager) orchestrateFrozen() {
	switch t.state.State {
	case instance.MonitorStateIdle,
		instance.MonitorStateStartFailed,
		instance.MonitorStateStopFailed,
		instance.MonitorStatePurgeFailed,
		instance.MonitorStateProvisionFailed,
		instance.MonitorStateUnprovisionFailed,
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
	t.doTransitionAction(t.freeze, instance.MonitorStateFreezing, instance.MonitorStateIdle, instance.MonitorStateFreezeFailed)
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
