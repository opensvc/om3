package imon

import (
	"context"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
)

func (t *Manager) orchestrateStarted() {
	if t.isStarted() {
		t.startedClearIfReached()
		return
	}
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.startedFromIdle()
	case instance.MonitorStateThawed:
		t.startedFromThawed()
	case instance.MonitorStateReady:
		t.startedFromReady()
	case instance.MonitorStateStarted:
		t.startedFromStarted()
	case instance.MonitorStateStartFailed:
		t.startedFromStartFailed()
	case instance.MonitorStateStarting:
		t.startedFromAny()
	case instance.MonitorStateStopping:
		t.startedFromAny()
	case instance.MonitorStateThawing:
	case instance.MonitorStateRunning:
	case instance.MonitorStateWaitParents:
		t.setWaitParents()
	default:
		t.log.Errorf("don't know how to orchestrate started from %s", t.state.State)
	}
}

// startedFromIdle handle global expect started orchestration from idle
//
// frozen => try startedFromFrozen
// else   => try startedFromThawed
func (t *Manager) startedFromIdle() {
	if t.instStatus[t.localhost].IsFrozen() {
		if t.state.GlobalExpect == instance.MonitorGlobalExpectNone {
			return
		}
		t.doUnfreeze()
		return
	} else {
		t.startedFromThawed()
	}
}

// startedFromThawed
//
// local started => unset global expect, set local expect started
// objectStatus.Avail Up => unset global expect, unset local expect
// better candidate => no actions
// else => state -> ready, start ready routine
func (t *Manager) startedFromThawed() {
	if t.startedClearIfReached() {
		return
	}
	if !t.state.IsHALeader {
		return
	}
	if t.hasOtherNodeActing() {
		t.log.Debugf("another node acting")
		return
	}
	if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.False, provisioned.Undef) {
		t.log.Debugf("provisioned is false or undef")
		return
	}
	t.transitionTo(instance.MonitorStateReady)
	t.createPendingWithDuration(t.readyDuration)
	go func(ctx context.Context) {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				return
			}
			t.orchestrateAfterAction(instance.MonitorStateReady, instance.MonitorStateReady)
			return
		}
	}(t.pendingCtx)
}

// doUnfreeze idle -> thawing -> thawed or thawed failed
func (t *Manager) doUnfreeze() {
	t.doTransitionAction(t.unfreeze, instance.MonitorStateThawing, instance.MonitorStateThawed, instance.MonitorStateThawedFailed)
}

func (t *Manager) cancelReadyState() bool {
	if t.pendingCancel == nil {
		t.loggerWithState().Errorf("startedFromReady without pending")
		t.transitionTo(instance.MonitorStateIdle)
		return true
	}
	if t.startedClearIfReached() {
		return true
	}
	if !t.state.IsHALeader {
		t.loggerWithState().Infof("leadership lost, clear the ready state")
		t.transitionTo(instance.MonitorStateIdle)
		t.clearPending()
		return true
	}
	return false
}

func (t *Manager) startedFromReady() {
	if isCanceled := t.cancelReadyState(); isCanceled {
		return
	}
	if t.setWaitParents() {
		return
	}
	select {
	case <-t.pendingCtx.Done():
		defer t.clearPending()
		if t.pendingCtx.Err() == context.Canceled {
			t.transitionTo(instance.MonitorStateIdle)
			return
		}
		t.doAction(t.crmStart, instance.MonitorStateStarting, instance.MonitorStateStarted, instance.MonitorStateStartFailed)
		return
	default:
		return
	}
}

func (t *Manager) startedFromAny() {
	if t.pendingCancel == nil {
		t.startedClearIfReached()
		return
	}
}

func (t *Manager) startedFromStarted() {
	t.startedClearIfReached()
}

func (t *Manager) startedFromStartFailed() {
	if t.isStarted() {
		t.loggerWithState().Infof("object is up -> set done and idle, clear start failed")
		t.doneAndIdle()
		return
	}
	if t.isAllStartFailed() {
		t.loggerWithState().Infof("all instances start failed -> set done")
		t.done()
		return
	}
}

func (t *Manager) isAllStartFailed() bool {
	for _, instMon := range t.AllInstanceMonitors() {
		if instMon.State != instance.MonitorStateStartFailed {
			return false
		}
	}
	return true
}

func (t *Manager) startedClearIfReached() bool {
	if t.isLocalStarted() {
		if !t.state.OrchestrationIsDone {
			t.loggerWithState().Infof("instance is started -> set done and idle")
			t.doneAndIdle()
		}
		if t.state.LocalExpect != instance.MonitorLocalExpectStarted {
			t.loggerWithState().Infof("instance is started, set local expect started")
			t.change = true
			t.state.LocalExpect = instance.MonitorLocalExpectStarted
		}
		t.clearPending()
		return true
	}
	if t.isStarted() {
		if !t.state.OrchestrationIsDone {
			t.loggerWithState().Infof("object is started -> set done and idle")
			t.doneAndIdle()
		}
		t.clearPending()
		return true
	}
	return false
}

func (t *Manager) isLocalStarted() bool {
	instStatus := t.instStatus[t.localhost]
	switch instStatus.Avail {
	case status.NotApplicable:
		return true
	case status.Up:
		return true
	case status.Undef:
		return false
	default:
		return false
	}
}
