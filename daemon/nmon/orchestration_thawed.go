package nmon

import "github.com/opensvc/om3/core/node"

func (t *Manager) orchestrateThawed() {
	switch t.state.State {
	case node.MonitorStateIdle:
		t.ThawedFromIdle()
	case node.MonitorStateThawing,
		node.MonitorStateThawedFailed:
	case node.MonitorStateThawed:
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
	t.doTransitionAction(t.crmUnfreeze, node.MonitorStateThawing, node.MonitorStateThawed, node.MonitorStateThawedFailed)
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
	if nodeStatus := node.StatusData.GetByNode(t.localhost); nodeStatus != nil && nodeStatus.FrozenAt.IsZero() {
		t.log.Infof("instance state is thawed, unset global expect")
		t.change = true
		t.state.GlobalExpect = node.MonitorGlobalExpectNone
		t.clearPending()
		return true
	}
	return false
}
