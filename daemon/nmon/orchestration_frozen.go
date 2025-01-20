package nmon

import "github.com/opensvc/om3/core/node"

func (t *Manager) orchestrateFrozen() {
	switch t.state.State {
	case node.MonitorStateIdle:
		t.frozenFromIdle()
	}
}

func (t *Manager) frozenFromIdle() {
	if t.frozenClearIfReached() {
		return
	}
	t.state.State = node.MonitorStateFreezing
	t.updateIfChange()
	t.log.Infof("run action freeze")
	nextState := node.MonitorStateIdle
	if err := t.crmFreeze(); err != nil {
		nextState = node.MonitorStateFreezeFailed
	}
	go t.orchestrateAfterAction(node.MonitorStateFreezing, nextState)
	return
}

func (t *Manager) frozenClearIfReached() bool {
	if nodeStatus := node.StatusData.GetByNode(t.localhost); nodeStatus != nil && !nodeStatus.FrozenAt.IsZero() {
		t.log.Infof("instance state is frozen, unset global expect")
		t.change = true
		t.state.GlobalExpect = node.MonitorGlobalExpectNone
		t.clearPending()
		return true
	}
	return false
}
