package nmon

import "github.com/opensvc/om3/v3/core/node"

func (t *Manager) orchestrateFrozen() {
	switch t.state.State {
	case node.MonitorStateIdle:
		t.frozenFromIdle()
	case node.MonitorStateFreezeSuccess:
		t.frozenFromFrozen()
	case node.MonitorStateUnfreezeSuccess:
		// A thaw done here, the node not back to idle yet: a freeze asked
		// meanwhile starts from idle.
		t.log.Infof("freeze asked as the thaw is done here: back to idle")
		t.state.State = node.MonitorStateIdle
		t.change = true
		t.frozenFromIdle()
	}
}

func (t *Manager) frozenFromIdle() {
	if t.frozenClearIfReached() {
		return
	}
	t.log.Infof("run action freeze")
	t.doTransitionAction(t.crmFreezeCluster, node.MonitorStateFreezeProgress, node.MonitorStateFreezeSuccess, node.MonitorStateFreezeFailure)
	return
}

func (t *Manager) frozenFromFrozen() {
	if t.frozenClearIfReached() {
		t.state.State = node.MonitorStateIdle
		t.change = true
		return
	}
	return
}

func (t *Manager) frozenClearIfReached() bool {
	if !t.nodeStatus.FrozenAt.IsZero() {
		t.log.Infof("node is frozen, unset global expect")
		t.change = true
		t.state.GlobalExpect = node.MonitorGlobalExpectNone
		t.clearPending()
		t.endOrchestration()
		return true
	}
	return false
}
