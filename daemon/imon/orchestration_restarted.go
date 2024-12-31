package imon

/*
   +------------------------------+
   |              idle            |
   +------------------------------+
      ^             |          |
      |             |          |
      |             |          |
      |             v          |
      |      +-------------+   |
      |      | wait priors |   |
      |      +-------------+   |
      |             |          |
      |             |          |
      |             |          |
      |             v          v
      |       +------------------+
      |       |   ready          |
      |       +------------------+
      |             |
      |             |
      |             |
      |             v
      |      +-------------+          +---------------+
      |      |   stopping  |--------->|  stop failed  |
      |      +-------------+          +---------------+
      |             |
      |             |
      |             |
      |             v
      |       +-----------+
      |       |  stopped  |
      |       +-----------+
      |             |
      |             |
      |             |
      |             v
      |      +------------+           +----------------+
      |      |  starting  |---------->|  start failed  |
      |      +------------+           +----------------+
      |             |
      |             |
      |             |
      |             v
      |      +-------------+
      +------|  restarted  |
             +-------------+

*/

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
	case instance.MonitorStateShutdown:
		t.orchestrateRestartedOnShutdown()
	case instance.MonitorStateStopped:
		t.orchestrateRestartedOnStopped()
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
		candidates = append(candidates, candidate)
	}
	return candidates
}

func (t *Manager) restartedOptions() (options instance.MonitorGlobalExpectOptionsRestarted) {
	options, _ = t.state.GlobalExpectOptions.(instance.MonitorGlobalExpectOptionsRestarted)
	return
}

func (t *Manager) orchestrateRestartedOnIdle() {
	if instanceStatus, ok := t.instStatus[t.localhost]; ok {
		switch instanceStatus.Avail {
		case status.Warn, status.Up:
		case status.StandbyUp:
			if !t.restartedOptions().Force {
				t.log.Infof("local instance initial avail is %s and restart is not forced -> set done",
					instanceStatus.Avail)
				t.done()
				return
			}
		default:
			t.log.Infof("local instance initial avail is %s -> set done", instanceStatus.Avail)
			t.done()
			return
		}
	}

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

func (t *Manager) orchestrateRestartedOnReady() {
	if instanceStatus, ok := t.instStatus[t.localhost]; ok {
		switch instanceStatus.Avail {
		case status.Warn, status.Up, status.StandbyUp:
			t.enableMonitor("all prior instances are restarted, ready to restart")
			t.createPendingWithDuration(stopDuration)
			if t.restartedOptions().Force {
				t.queueAction(t.crmShutdown, instance.MonitorStateShutting, instance.MonitorStateShutdown, instance.MonitorStateShutdownFailed)
			} else {
				t.queueAction(t.crmStop, instance.MonitorStateStopping, instance.MonitorStateStopped, instance.MonitorStateStopFailed)
			}
		default:
			t.log.Infof("all prior instances are restarted, local instance avail is %s -> done", instanceStatus.Avail)
			t.doneAndIdle()
		}
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
		if instanceMonitor.State == instance.MonitorStateRestarted || instanceMonitor.OrchestrationIsDone {
			continue
		}
		t.log.Debugf("prior instance on %s is not done nor restarted yet (%s), keep waiting", nodename, instanceMonitor.State)
		return
	}
	t.log.Infof("no prior instances, ready to restart")
	t.state.State = instance.MonitorStateReady
	t.change = true
	t.priors = []string{}
}

func (t *Manager) orchestrateRestartedOnStopped() {
	t.doTransitionAction(t.crmStart, instance.MonitorStateStarting, instance.MonitorStateRestarted, instance.MonitorStateStartFailed)
}

func (t *Manager) orchestrateRestartedOnShutdown() {
	t.doTransitionAction(t.crmStartStandby, instance.MonitorStateStarting, instance.MonitorStateRestarted, instance.MonitorStateStartFailed)
}

func (t *Manager) orchestrateRestartedOnRestarted() {
	for nodename, instanceMonitor := range t.instMonitor {
		if instanceMonitor.OrchestrationIsDone {
			continue
		}
		switch instanceMonitor.State {
		case instance.MonitorStateRestarted:
			continue
		case instance.MonitorStateStartFailed:
			continue
		case instance.MonitorStateStopFailed:
			continue
		}
		t.loggerWithState().Infof("instance on %s state is %s -> wait", nodename, instanceMonitor.State)
		return
	}
	t.enableMonitor("all instances are restarted, failed or orchestrate done")
	t.doneAndIdle()
	t.clearPending()
}
