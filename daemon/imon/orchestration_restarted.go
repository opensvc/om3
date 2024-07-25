package imon

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/util/stringslice"
)

func (t *Manager) orchestrateRestarted() {
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.orchestrateRestartedOnIdle()
	case instance.MonitorStateWaitPriors:
		t.orchestrateRestartedOnWaitPriors()
	case instance.MonitorStateReady:
		t.orchestrateRestartedOnReady()
	case instance.MonitorStateInterrupted:
		t.orchestrateRestartedOnInterrupted()
	case instance.MonitorStateRestarted:
		t.orchestrateRestartedOnRestarted()
	case instance.MonitorStateFrozen:
	case instance.MonitorStateFreezing:
	case instance.MonitorStateRunning:
	case instance.MonitorStateStopping:
	case instance.MonitorStateStopFailed:
	case instance.MonitorStateStarting:
	case instance.MonitorStateStartFailed:
	default:
		t.log.Errorf("don't know how to restart from %s", t.state.State)
	}
}

func (t *Manager) getPriors() []string {
	candidates := make([]string, 0)

	for _, candidate := range t.sortCandidates(t.scopeNodes) {
		if t.localhost == candidate {
			return candidates
		}
		if instStatus, ok := t.instStatus[candidate]; ok {
			switch instStatus.Avail {
			case status.Up, status.Warn:
				candidates = append(candidates, candidate)
			}
		}
	}
	return candidates
}

func (t *Manager) orchestrateRestartedOnIdle() {
	t.priors = t.getPriors()
	priorsLength := len(t.priors)
	switch priorsLength {
	case 0:
		t.log.Infof("no prior instances, ready to restart")
		t.state.State = instance.MonitorStateReady
		t.change = true
	default:
		t.log.Infof("wait prior instance %s to be restarted", t.priors)
		t.state.State = instance.MonitorStateWaitPriors
		t.change = true
	}
}

func (t *Manager) orchestrateRestartedOnWaitPriors() {
	for _, nodename := range t.priors {
		instanceMonitor, ok := t.instMonitor[nodename]
		if !ok {
			t.log.Debugf("skip prior instance on %s: no instance monitor data", nodename)
			t.priors = stringslice.Remove(t.priors, nodename)
			continue
		}
		if instanceMonitor.State == instance.MonitorStateRestarted {
			continue
		}
		t.log.Debugf("prior instance on %s is not restarted yet, keep waiting", nodename)
		return
	}
	t.log.Infof("all prior instances are restarted, ready to restart")
	t.state.State = instance.MonitorStateReady
	t.change = true
	t.priors = []string{}
}

func (t *Manager) orchestrateRestartedOnInterrupted() {
	t.doTransitionAction(t.crmStart, instance.MonitorStateStarting, instance.MonitorStateRestarted, instance.MonitorStateStartFailed)
}

func (t *Manager) orchestrateRestartedOnReady() {
	t.state.LocalExpect = instance.MonitorLocalExpectStarted
	t.change = true
	t.createPendingWithDuration(stopDuration)
	t.queueAction(t.crmStop, instance.MonitorStateStopping, instance.MonitorStateInterrupted, instance.MonitorStateStopFailed)
}

func (t *Manager) orchestrateRestartedOnRestarted() {
	if t.state.OrchestrationIsDone {
		return
	}
	t.loggerWithState().Infof("instance state is restarted -> set done and idle, clear local expect")
	t.doneAndIdle()
	t.state.LocalExpect = instance.MonitorLocalExpectStarted
	t.clearPending()
}
