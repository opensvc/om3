package nmon

import "github.com/opensvc/om3/v3/core/node"

func (t *Manager) orchestrateUnfrozen() {
	switch t.state.State {
	case node.MonitorStateIdle:
		t.unfrozenFromIdle()
	case node.MonitorStateUnfreezeProgress,
		node.MonitorStateUnfreezeFailure:
	case node.MonitorStateUnfreezeSuccess:
		t.unfrozenFromUnfreezeSuccess()
	case node.MonitorStateFreezeSuccess:
		// A freeze done here, the node not back to idle yet as its
		// orchestration ended elsewhere first: a thaw asked meanwhile
		// found no way from there, and the node stayed frozen for good.
		t.log.Infof("thaw asked as the freeze is done here: back to idle")
		t.state.State = node.MonitorStateIdle
		t.change = true
		t.unfrozenFromIdle()
	default:
		t.log.Warnf("don't know how to orchestrate %s from %s", t.state.GlobalExpect, t.state.State)
	}
}

func (t *Manager) unfrozenFromIdle() {
	if t.unfrozenClearIfReached() {
		return
	}
	t.log.Infof("run action unfreeze")
	t.doTransitionAction(t.crmUnfreeze, node.MonitorStateUnfreezeProgress, node.MonitorStateUnfreezeSuccess, node.MonitorStateUnfreezeFailure)
	return
}

func (t *Manager) unfrozenFromUnfreezeSuccess() {
	if t.unfrozenClearIfReached() {
		t.state.State = node.MonitorStateIdle
		t.change = true
		t.updateIfChange()
		return
	}
	return
}

func (t *Manager) unfrozenClearIfReached() bool {
	if t.nodeStatus.FrozenAt.IsZero() {
		t.log.Infof("node is no longer frozen, unset global expect")
		t.change = true
		t.state.GlobalExpect = node.MonitorGlobalExpectNone
		t.clearPending()
		t.endOrchestration()
		return true
	}
	return false
}
