package imon

import (
	"context"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/topology"
)

func (t *Manager) orchestrateStarted() {
	if t.isStarted() {
		t.startedClearIfReached()
		return
	}
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.startedFromIdle()
	case instance.MonitorStateUnfreezeSuccess:
		t.startedFromUnfrozen()
	case instance.MonitorStateReady:
		t.startedFromReady()
	case instance.MonitorStateStartSuccess:
		t.startedFromStarted()
	case instance.MonitorStateStartFailure:
		t.startedFromStartFailed()
	case instance.MonitorStateStartProgress:
		t.startedFromAny()
	case instance.MonitorStateStopProgress:
		t.startedFromAny()
	case instance.MonitorStateUnfreezeProgress:
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
// else   => try startedFromUnfrozen
func (t *Manager) startedFromIdle() {
	if t.instStatus[t.localhost].IsFrozen() {
		if t.state.GlobalExpect == instance.MonitorGlobalExpectNone {
			return
		}
		t.doUnfreeze()
		return
	} else {
		t.startedFromUnfrozen()
	}
}

// startedFromUnfrozen
//
// local started => unset global expect, set local expect started
// objectStatus.Avail Up => unset global expect, unset local expect
// better candidate => no actions
// else => state -> ready, start ready routine
func (t *Manager) startedFromUnfrozen() {
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
	if t.objStatus.Topology != topology.Flex {
		if nodename, state := t.isAnyPeerState(instance.MonitorStateStartProgress, instance.MonitorStateReady); nodename != "" {
			t.log.Debugf("peer %s imon state is %s", nodename, state)
			return
		}
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

// doUnfreeze idle -> unfreezing -> unfrozen or unfreeze failed
func (t *Manager) doUnfreeze() {
	t.doTransitionAction(t.unfreeze, instance.MonitorStateUnfreezeProgress, instance.MonitorStateUnfreezeSuccess, instance.MonitorStateUnfreezeFailure)
}

// cancelReadyState transitions the monitor to an Idle state if certain
// conditions are met, clearing pending states as needed.
// conditions to return idle:
// - if locally started (startedClearIfReached)
// - leadership is lost
// - topology is flex and found peer instance that is starting or ready
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
	if t.objStatus.Topology != topology.Flex {
		if nodename, state := t.isAnyPeerState(instance.MonitorStateStartProgress, instance.MonitorStateReady); nodename != "" {
			t.loggerWithState().Infof("peer %s imon state is %s, clear the ready state", nodename, state)
			t.transitionTo(instance.MonitorStateIdle)
			t.clearPending()
			return true
		}
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
		t.queueAction(t.crmStart, instance.MonitorStateStartProgress, instance.MonitorStateStartSuccess, instance.MonitorStateStartFailure)
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
	if t.state.OrchestrationIsDone {
		return
	}
	if t.isAllState(instance.MonitorStateStartFailure) {
		t.loggerWithState().Infof("all instances start failed -> set done")
		t.done()
		return
	}
}

func (t *Manager) isAnyPeerState(states ...instance.MonitorState) (string, instance.MonitorState) {
	for nodename, instMon := range t.AllInstanceMonitors() {
		if nodename == t.localhost {
			continue
		}
		for _, state := range states {
			if instMon.State == state {
				return nodename, state
			}
		}
	}
	return "", instance.MonitorStateInit
}

func (t *Manager) isAllState(state ...instance.MonitorState) bool {
	for _, instMon := range t.AllInstanceMonitors() {
		if !instMon.State.Is(state...) {
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
		t.enableMonitor("local instance is started")
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
