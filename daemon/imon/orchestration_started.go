package imon

import (
	"context"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
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

// startedFromIdle handles a started orchestration from idle.
//
// A frozen instance is not started by the daemon on its own: freezing is how
// an operator says the daemon may not act by itself, and nobody asked for
// this one. An instance a stop flagged stopped on purpose is not started
// either, for the same reason: the operator asked for it to be down.
//
// Both are started when a user asks for it, and both flags are left as they
// were found. The freeze used to be lifted first, which is neither honouring
// the request nor refusing it, and silently discarded what the operator had
// set. The stopped flag is not lifted here but where the request arrives, on
// every instance of the object, so the instances that stay down are
// candidates for a later failover.
func (t *Manager) startedFromIdle() {
	if t.state.GlobalExpect == instance.MonitorGlobalExpectNone {
		if t.instStatus[t.localhost].IsFrozen() {
			return
		}
		if t.isStopped() {
			return
		}
	}
	t.startedFromUnfrozen()
}

// isStartLeader says the local instance is the one to start.
//
// A start the daemon decided on by itself asks the HA leader rule, which
// passes over the frozen instances: freezing is how an operator says the
// daemon may not act by itself.
//
// A start a user asked for asks the same rule without the frozen exclusion,
// so a frozen instance is started without its freeze being touched. The rest
// of the rule is kept, because being unprovisioned, unrankable or start
// failed says the instance cannot start whoever is asking.
//
// The start an orchestrate=start object is due when its node boots asks the
// natural placement leader instead. The ha rule hands the object to a peer
// when this node cannot take it, which is a failover, and the promise of
// orchestrate=start is that the object is started where it belongs and moved
// nowhere.
func (t *Manager) isStartLeader() bool {
	switch {
	case t.state.GlobalExpect == instance.MonitorGlobalExpectStarted:
		return t.isStartCandidateLeader(false)
	case t.objStatus.Orchestrate == "start":
		return t.state.IsLeader
	default:
		return t.state.IsHALeader
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
	if !t.isStartLeader() {
		return
	}
	if t.hasOtherNodeActing() {
		t.log.Tracef("another node acting")
		return
	}
	if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.False, provisioned.Undef) {
		t.log.Tracef("provisioned is false or undef")
		return
	}
	if t.objStatus.Topology != topology.Flex {
		if nodename, state := t.isAnyPeerState(instance.MonitorStateStartProgress, instance.MonitorStateReady); nodename != "" {
			t.log.Tracef("peer %s imon state is %s", nodename, state)
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
	if !t.isStartLeader() {
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

	stonith := func() error {
		if t.peerDrop == "" {
			return nil
		}
		if t.state.GlobalExpect != instance.MonitorGlobalExpectNone {
			t.log.Infof("stonith: %s: skip because global expect is set (%s)", t.peerDrop, t.state.GlobalExpect)
			return nil
		}
		nodeStonithAtMapMutex.Lock()

		defer func() {
			nodeStonithAtMapMutex.Unlock()
			t.unsetStonith()
		}()

		nodeStonithAt, ok := nodeStonithAtMap[t.peerDrop]
		if ok && !nodeStonithAt.Before(t.peerDropAt) {
			t.log.Tracef("stonith: %s already fenced", t.peerDrop)
			return nil
		}
		nodeStonithAtMap[t.peerDrop] = t.peerDropAt
		t.log.Infof("stonith: %s: fence", t.peerDrop)
		return t.crmStonith(t.peerDrop)
	}

	stonithAndStart := func() error {
		if err := stonith(); err != nil {
			t.log.Warnf("stonith: %s: %s", t.peerDrop, err)
		}
		return t.crmStart()
	}

	select {
	case <-t.pendingCtx.Done():
		defer t.clearPending()
		if t.pendingCtx.Err() == context.Canceled {
			t.transitionTo(instance.MonitorStateIdle)
			return
		}
		t.queueAction(stonithAndStart, instance.MonitorStateStartProgress, instance.MonitorStateStartSuccess, instance.MonitorStateStartFailure)
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
		if !instMon.State.IsOneOf(state...) {
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
