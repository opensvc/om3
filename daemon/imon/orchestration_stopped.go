package imon

import (
	"time"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/status"
)

var (
	stopDuration = 10 * time.Second
)

func (t *Manager) orchestrateStopped() {
	t.freezeStop()
}

func (t *Manager) freezeStop() {
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.doFreezeStop()
	case instance.MonitorStateFrozen:
		t.doStop()
	case instance.MonitorStateReady:
		t.stoppedFromReady()
	case instance.MonitorStateFreezing:
		// wait for the freeze exec to end
	case instance.MonitorStateRunning:
	case instance.MonitorStateStopped:
		t.transitionTo(instance.MonitorStateIdle)
	case instance.MonitorStateStopping:
		// avoid multiple concurrent stop execs
	case instance.MonitorStateStopFailed:
		// avoid a retry-loop
	case instance.MonitorStateStartFailed:
		t.stoppedFromFailed()
	case instance.MonitorStateWaitChildren:
		t.setWaitChildren()
	default:
		t.log.Errorf("don't know how to freeze and stop from %s", t.state.State)
	}
}

// stop stops the object but does not freeze.
// This func must be called by orchestrations that know the ha auto-start will
// not starts it back (ex: auto-stop), or that want the restart (ex: restart).
func (t *Manager) stop() {
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.doStop()
	case instance.MonitorStateReady:
		t.stoppedFromReady()
	case instance.MonitorStateStopped:
		t.transitionTo(instance.MonitorStateIdle)
	case instance.MonitorStateFrozen:
		// honor the frozen state
	case instance.MonitorStateFreezing:
		// wait for the freeze exec to end
	case instance.MonitorStateStopping:
		// avoid multiple concurrent stop execs
	case instance.MonitorStateStopFailed:
		// avoid a retry-loop
	case instance.MonitorStateStartFailed:
		t.stoppedFromFailed()
	default:
		t.log.Errorf("don't know how to stop from %s", t.state.State)
	}
}

func (t *Manager) stoppedFromThawed() {
	t.doTransitionAction(t.freeze, instance.MonitorStateFreezing, instance.MonitorStateIdle, instance.MonitorStateFreezeFailed)
}

// doFreeze handle global expect stopped orchestration from idle
//
// local thawed => freezing to reach frozen
// else         => stopping
func (t *Manager) doFreezeStop() {
	if t.instStatus[t.localhost].IsThawed() {
		t.doTransitionAction(t.freeze, instance.MonitorStateFreezing, instance.MonitorStateFrozen, instance.MonitorStateFreezeFailed)
		return
	} else {
		t.doStop()
	}
}

func (t *Manager) doFreeze() {
	if t.instStatus[t.localhost].IsThawed() {
		t.doTransitionAction(t.freeze, instance.MonitorStateFreezing, instance.MonitorStateFrozen, instance.MonitorStateFreezeFailed)
		return
	}
}

func (t *Manager) doStop() {
	if t.stoppedClearIfReached() {
		return
	}
	if t.setWaitChildren() {
		return
	}
	t.createPendingWithDuration(stopDuration)
	t.queueAction(t.crmStop, instance.MonitorStateStopping, instance.MonitorStateStopped, instance.MonitorStateStopFailed)
}

func (t *Manager) stoppedFromReady() {
	t.log.Infof("reset ready state global expect is stopped")
	t.clearPending()
	t.change = true
	t.state.State = instance.MonitorStateIdle
	t.stoppedClearIfReached()
}

func (t *Manager) stoppedFromFailed() {
	t.log.Infof("reset %s state global expect is stopped")
	t.change = true
	t.state.State = instance.MonitorStateIdle
	t.stoppedClearIfReached()
}

func (t *Manager) stoppedFromAny() {
	if t.pendingCancel == nil {
		t.stoppedClearIfReached()
		return
	}
}

func (t *Manager) stoppedClearIfReached() bool {
	if t.isLocalStopped() {
		if !t.state.OrchestrationIsDone {
			t.loggerWithState().Infof("instance state is stopped -> set done and idle, clear local expect")
			t.doneAndIdle()
			t.state.LocalExpect = instance.MonitorLocalExpectNone
			t.clearPending()
		}
		return true
	}
	return false
}

func (t *Manager) isLocalStopped() bool {
	instStatus := t.instStatus[t.localhost]
	switch instStatus.Avail {
	case status.NotApplicable, status.Undef:
		return true
	case status.Down:
		return true
	case status.StandbyUp:
		return true
	case status.StandbyDown:
		return true
	default:
		return false
	}
}
