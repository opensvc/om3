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
	case instance.MonitorStateFreezeSuccess:
		t.doStop()
	case instance.MonitorStateReady:
		t.stoppedFromReady()
	case instance.MonitorStateFreezeProgress:
		// wait for the freeze exec to end
	case instance.MonitorStateRunning:
	case instance.MonitorStateStopSuccess:
		t.transitionTo(instance.MonitorStateIdle)
	case instance.MonitorStateStopProgress:
		// avoid multiple concurrent stop execs
	case instance.MonitorStateStopFailure:
		t.done()
	case instance.MonitorStateStartFailure:
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
	case instance.MonitorStateStopSuccess:
		t.transitionTo(instance.MonitorStateIdle)
	case instance.MonitorStateFreezeSuccess:
		// honor the frozen state
	case instance.MonitorStateFreezeProgress:
		// wait for the freeze exec to end
	case instance.MonitorStateStopProgress:
		// avoid multiple concurrent stop execs
	case instance.MonitorStateStopFailure:
		// avoid a retry-loop
	case instance.MonitorStateStartFailure:
		t.stoppedFromFailed()
	default:
		t.log.Errorf("don't know how to stop from %s", t.state.State)
	}
}

// doFreeze handle global expect stopped orchestration from idle
//
// local unfrozen => freezing to reach frozen
// else           => stopping
func (t *Manager) doFreezeStop() {
	if t.instStatus[t.localhost].IsUnfrozen() {
		t.doTransitionAction(t.freeze, instance.MonitorStateFreezeProgress, instance.MonitorStateFreezeSuccess, instance.MonitorStateFreezeFailure)
		return
	} else {
		t.doStop()
	}
}

func (t *Manager) doFreeze() {
	if t.instStatus[t.localhost].IsUnfrozen() {
		t.doTransitionAction(t.freeze, instance.MonitorStateFreezeProgress, instance.MonitorStateFreezeSuccess, instance.MonitorStateFreezeFailure)
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
	t.disableMonitor("orchestrate stop")
	t.queueAction(t.crmStop, instance.MonitorStateStopProgress, instance.MonitorStateStopSuccess, instance.MonitorStateStopFailure)
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
			t.loggerWithState().Infof("instance state is stopped -> set done and idle")
			t.doneAndIdle()
			t.disableMonitor("orchestrate stop from instance stopped")
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
