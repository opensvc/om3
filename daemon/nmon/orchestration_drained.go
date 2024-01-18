package nmon

import (
	"time"

	"github.com/opensvc/om3/core/node"
)

func (t *Manager) orchestrateDrained() {
	switch t.state.State {
	case node.MonitorStateIdle:
		t.drainFromIdle()
	case node.MonitorStateFrozen:
		t.drainFromFrozen()
	case node.MonitorStateDrained:
		t.change = true
		t.state.State = node.MonitorStateIdle
		t.state.LocalExpect = node.MonitorLocalExpectNone
	default:
		t.log.Warnf("orchestrate drained no solution from state %s", t.state.State)
		time.Sleep(unexpectedDelay)
	}
}

func (t *Manager) drainFromIdle() {
	if nodeStatus := node.StatusData.Get(t.localhost); nodeStatus != nil && !nodeStatus.FrozenAt.IsZero() {
		// already frozen, ... advance to "frozen" state
		t.state.State = node.MonitorStateFrozen
		go func() {
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateFrozen, newState: node.MonitorStateFrozen}
		}()
		return
	}

	// freeze
	t.change = true
	t.state.State = node.MonitorStateFreezing
	t.updateIfChange()
	go func() {
		t.log.Infof("run action freeze")
		if err := t.crmFreeze(); err != nil {
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateFreezing, newState: node.MonitorStateFreezeFailed}
		} else {
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateFreezing, newState: node.MonitorStateFrozen}
		}
	}()
	return
}

func (t *Manager) drainFromFrozen() {
	t.change = true
	t.state.State = node.MonitorStateDraining
	t.updateIfChange()
	go func() {
		t.log.Infof("run shutdown action on all local instances")
		if err := t.crmDrain(); err != nil {
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateDraining, newState: node.MonitorStateDrainFailed}
		} else {
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateDraining, newState: node.MonitorStateDrained}
		}
	}()
	return
}
