package nmon

import (
	"time"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/util/hostname"
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
	case node.MonitorStateDrainFailed:
		t.change = true
		t.state.LocalExpect = node.MonitorLocalExpectNone
	default:
		t.log.Warnf("orchestrate drained no solution from state %s", t.state.State)
		time.Sleep(unexpectedDelay)
	}
}

func (t *Manager) drainFromIdle() {
	if nodeStatus := node.StatusData.GetByNode(t.localhost); nodeStatus != nil && !nodeStatus.FrozenAt.IsZero() {
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
		if !hasLocalKind(naming.KindSvc) {
			// don't try crmDrain when no local */svc/* object exists
			t.log.Infof("no local instance to shutdown")
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateDraining, newState: node.MonitorStateDrained}
			return
		}
		t.log.Infof("run shutdown action on all local instances")
		if err := t.crmDrain(); err != nil {
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateDraining, newState: node.MonitorStateDrainFailed}
		} else {
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateDraining, newState: node.MonitorStateDrained}
		}
	}()
	return
}

func hasLocalKind(k naming.Kind) bool {
	localInstanceConfig := instance.ConfigData.GetByNode(hostname.Hostname())
	for p := range localInstanceConfig {
		p.Kind.Or()
		if p.Kind == naming.KindSvc {
			return true
		}
	}
	return false
}
