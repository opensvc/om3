package nmon

import "github.com/opensvc/om3/core/node"

func (t *Manager) orchestrateThawed() {
	switch t.state.State {
	case node.MonitorStateIdle:
		t.ThawedFromIdle()
	case node.MonitorStateThawProgress,
		node.MonitorStateThawFailure:
	case node.MonitorStateThawSuccess:
		t.thawedFromThawed()
	default:
		t.log.Warnf("don't know how to orchestrate %s from %s", t.state.GlobalExpect, t.state.State)
	}
}

func (t *Manager) ThawedFromIdle() {
	if t.thawedClearIfReached() {
		return
	}
	t.log.Infof("run action unfreeze")
	t.doTransitionAction(t.crmUnfreeze, node.MonitorStateThawProgress, node.MonitorStateThawSuccess, node.MonitorStateThawFailure)
	return
}

func (t *Manager) thawedFromThawed() {
	if t.thawedClearIfReached() {
		t.state.State = node.MonitorStateIdle
		t.change = true
		t.updateIfChange()
		return
	}
	return
}

func (t *Manager) thawedClearIfReached() bool {
	if t.nodeStatus.FrozenAt.IsZero() {
		t.log.Infof("node is no longer frozen, unset global expect")
		t.change = true
		t.state.GlobalExpect = node.MonitorGlobalExpectNone
		t.clearPending()
		return true
	}
	return false
}
