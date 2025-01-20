package nmon

import "github.com/opensvc/om3/core/node"

func (t *Manager) orchestrateThawed() {
	switch t.state.State {
	case node.MonitorStateIdle:
		t.ThawedFromIdle()
	case node.MonitorStateThawing:
	default:
		t.log.Warnf("don't know how to orchestrate %s from %s", t.state.GlobalExpect, t.state.State)
	}
}

func (t *Manager) ThawedFromIdle() {
	if t.thawedClearIfReached() {
		return
	}
	t.transitionTo(node.MonitorStateThawing)
	t.log.Infof("run action unfreeze")
	nextState := node.MonitorStateIdle
	if err := t.crmUnfreeze(); err != nil {
		nextState = node.MonitorStateThawedFailed
	}
	go t.orchestrateAfterAction(node.MonitorStateThawing, nextState)
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
