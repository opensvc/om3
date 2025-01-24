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
	case node.MonitorStateFreezeSuccess:
		t.drainFromFrozen()
	case node.MonitorStateDrainSuccess:
		t.change = true
		t.state.State = node.MonitorStateIdle
		t.state.LocalExpect = node.MonitorLocalExpectNone
	case node.MonitorStateDrainFailure:
		t.change = true
		t.state.LocalExpect = node.MonitorLocalExpectNone
	default:
		t.log.Warnf("orchestrate drained no solution from state %s", t.state.State)
		time.Sleep(unexpectedDelay)
	}
}

func (t *Manager) drainFromIdle() {
	if !t.nodeStatus.FrozenAt.IsZero() {
		// already frozen, ... advance to "frozen" state
		t.state.State = node.MonitorStateFreezeSuccess
		go func() {
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateFreezeSuccess, newState: node.MonitorStateFreezeSuccess}
		}()
		return
	}

	// freeze
	t.change = true
	t.state.State = node.MonitorStateFreezeProgress
	t.updateIfChange()
	go func() {
		t.log.Infof("run action freeze")
		if err := t.crmFreeze(); err != nil {
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateFreezeProgress, newState: node.MonitorStateFreezeFailure}
		} else {
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateFreezeProgress, newState: node.MonitorStateFreezeSuccess}
		}
	}()
	return
}

func (t *Manager) drainFromFrozen() {
	t.change = true
	t.state.State = node.MonitorStateDrainProgress
	t.updateIfChange()
	go func() {
		if !hasLocalKind(naming.KindSvc) {
			// don't try crmDrain when no local */svc/* object exists
			t.log.Infof("no local instance to shutdown")
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateDrainProgress, newState: node.MonitorStateDrainSuccess}
			return
		}
		t.log.Infof("run shutdown action on all local instances")
		if err := t.crmDrain(); err != nil {
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateDrainProgress, newState: node.MonitorStateDrainFailure}
		} else {
			t.cmdC <- cmdOrchestrate{state: node.MonitorStateDrainProgress, newState: node.MonitorStateDrainSuccess}
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
