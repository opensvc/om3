package imon

import (
	"fmt"
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
)

func (t *Manager) orchestrateNone() {
	t.clearStartFailed()
	t.clearBootFailed()
	t.clearStoppedFlagWhenUp()
	if t.objStatus.ActorStatus == nil {
		return
	}
	switch t.objStatus.Orchestrate {
	case "ha":
		t.orchestrateHAStart()
		t.orchestrateHAStop()
	case "start":
		t.orchestrateBootStart()
	}
}

// orchestrateBootStart starts the object on the first daemon start that
// follows a node boot, and only then.
//
// orchestrate=start says the daemon starts the object when its node comes up,
// and never moves it afterwards. The start is due when the object is below
// what it should be running: not up for a failover object, fewer up instances
// than flex_target for a flex one. Who takes it is the natural placement
// leader, and not the leader among the instances that could start now, which
// is what the ha orchestration asks: passing the object to a peer because
// this node cannot take it is a failover, which is the half of ha that
// orchestrate=start does not want.
//
// The decision waits for a view of the object complete enough to know whether
// it already runs elsewhere, and is taken once. Nothing left pending outlives
// the boot: an object this node did not have to start when it came up is not
// started here later.
func (t *Manager) orchestrateBootStart() {
	if !t.isBootStartPending {
		return
	}
	if t.nodeStatus[t.localhost].IsFrozen() {
		// The natural placement leader rule, unlike the ha leader rule, does
		// not pass over the frozen nodes: it ranks where the object belongs,
		// not who may start it. The node freeze is the operator saying the
		// daemon may not act here, and it outlives the reboot.
		t.closeBootStart("the node is frozen")
		return
	}
	switch t.state.State {
	case instance.MonitorStateIdle:
		if v, reason := t.hasInstanceMonitorAndStatusOnPeers(); !v {
			// the decision needs to know what the peers hold, or a node
			// coming up first would start what another one already runs
			t.log.Tracef("boot start: %s", reason)
			return
		}
		// A peer instance doing something, as the start of a standby
		// resource every node makes at its boot, or readying a start, is
		// passed over by the start rules: the decision waits for it to end
		// rather than be taken on it, which it would not be a second
		// later.
		if peer, state := t.bootStartBusyPeer(); peer != "" {
			if t.bootStartWaitSince.IsZero() {
				t.bootStartWaitSince = time.Now()
				t.log.Infof("boot start: wait for the %s instance, %s", peer, state)
			} else if time.Since(t.bootStartWaitSince) > bootStartWaitMax {
				t.closeBootStart("the %s instance is %s for longer than %s", peer, state, bootStartWaitMax)
			}
			return
		}
		if !t.bootStartWaitSince.IsZero() && time.Since(t.bootStartWaitSince) > bootStartWaitMax {
			t.closeBootStart("the peer instances were busy for longer than %s", bootStartWaitMax)
			return
		}
		t.orchestrateHAStart()
		switch {
		case t.state.State != instance.MonitorStateIdle:
			// the start is on its way
		case t.isStarted():
			t.closeBootStart("the object is started")
		default:
			t.closeBootStart("%s", t.bootStartNoopReason())
		}
	case instance.MonitorStateReady, instance.MonitorStateStartProgress,
		instance.MonitorStateStartSuccess, instance.MonitorStateStopSuccess:
		// the start this boot called for is on its way, and the same
		// orchestration sees it through
		t.orchestrateHAStart()
	case instance.MonitorStateStartFailure:
		t.closeBootStart("the start failed")
	default:
		t.closeBootStart("the instance is %s", t.state.State)
	}
}

// bootStartWaitMax bounds the wait of the boot start decision for the peer
// instances to end what they do: a start decided long after the node came up
// is not what orchestrate=start says.
var bootStartWaitMax = 5 * time.Minute

// bootStartBusyPeer returns a peer instance doing something, or readying a
// start, and its state, and "" when none is.
func (t *Manager) bootStartBusyPeer() (string, instance.MonitorState) {
	for peer, instMon := range t.instMonitor {
		if peer == t.localhost {
			continue
		}
		if instMon.State.IsDoing() || instMon.State == instance.MonitorStateReady {
			return peer, instMon.State
		}
	}
	return "", instance.MonitorStateIdle
}

// bootStartNoopReason says why the start orchestration started nothing on
// this node, for the boot start decision to say it.
func (t *Manager) bootStartNoopReason() string {
	local := t.instStatus[t.localhost]
	switch {
	case local.IsFrozen():
		return "the instance is frozen"
	case t.isStopped():
		return "the instance is flagged stopped"
	case !t.state.IsLeader:
		return "this node is not the placement leader"
	case local.Provisioned.IsOneOf(provisioned.False, provisioned.Undef):
		return fmt.Sprintf("the instance provisioned state is %s", local.Provisioned)
	case local.Avail.Is(status.Warn):
		return "the instance is warn"
	}
	if v, reason := t.isHAOrchestrateable(); !v {
		return reason
	}
	return "nothing for this node to start"
}

func (t *Manager) closeBootStart(format string, a ...any) {
	t.isBootStartPending = false
	t.bootStartWaitSince = time.Time{}
	t.log.Infof("boot start decided: "+format, a...)
}

func (t *Manager) orchestrateHAStop() {
	if t.objStatus.Topology != topology.Flex {
		return
	}

	if t.nodeStatus[t.localhost].IsFrozen() {
		return
	} else if t.objStatus.UpInstancesCount <= t.objStatus.Flex.Target {
		return
	} else if t.state.IsHALeader {
		return
	} else if v, _ := t.isHAOrchestrateable(); !v {
		return
	} else if t.objStatus.Avail != status.Up {
		return
	}

	t.stop()
}

func (t *Manager) orchestrateHAStart() {
	// we are here because we are ha object with global expect None
	switch t.state.State {
	case instance.MonitorStateReady:
		t.cancelReadyState()
	case instance.MonitorStateStartSuccess:
		// started means the action start has been done. This state is a
		// waiter step to verify if received started like local instance status
		// to transition state: started -> idle
		// It prevents unexpected transition state -> ready
		if t.isLocalStarted() {
			t.enableMonitor("instance is now started")
			t.transitionTo(instance.MonitorStateIdle)
		}
		return
	case instance.MonitorStateStopSuccess:
		// stopped means the action stop has been done. This state is a
		// waiter step to take time to disable the local expect started.
		t.disableMonitor("instance is now stopped")
		t.transitionTo(instance.MonitorStateIdle)
		return
	}
	if v, reason := t.isStartable(); !v {
		if t.pendingCancel != nil && t.state.State == instance.MonitorStateReady {
			t.log.Infof("instance is not startable, clear the ready state: %s", reason)
			t.clearPending()
			t.transitionTo(instance.MonitorStateIdle)
		}
		return
	}
	if t.isLocalStarted() {
		return
	}
	if t.instStatus[t.localhost].Avail.Is(status.Warn) {
		// must be skipped when instance status avail is warn,
		// this is handled by instance resource monitoring.
		// example: after instance stop --rid ...
		return
	}
	t.orchestrateStarted()
}

// clearBootFailed clears the boot failed state when the following conditions are met:
//
// + local avail is Down, StandbyDown, NotApplicable
// + global expect is none
func (t *Manager) clearBootFailed() {
	if t.state.State != instance.MonitorStateBootFailed {
		return
	}
	switch t.instStatus[t.localhost].Avail {
	case status.Down:
	case status.StandbyDown:
	case status.NotApplicable:
	default:
		return
	}
	for _, instanceMonitor := range t.instMonitor {
		switch instanceMonitor.GlobalExpect {
		case instance.MonitorGlobalExpectNone:
		default:
			return
		}
	}
	t.log.Infof("clear instance %s: local instance avail is %s, object avail is %s",
		t.state.State, t.instStatus[t.localhost].Avail, t.objStatus.Avail)
	t.transitionTo(instance.MonitorStateIdle)
}

// clearStoppedFlagWhenUp lowers the stopped flag when the instance is up, or
// when the object runs all it should.
//
// Whoever started the instance wants it up, so the daemon may keep it up:
// this is what re-arms an instance started outside of an orchestration, with
// "om <path> start --local" on a node whose daemon is down, for example. It
// mirrors the resource restart, which is re-armed when the resource is seen
// up again.
func (t *Manager) clearStoppedFlagWhenUp() {
	if !t.isStopped() {
		return
	}
	reason := stoppedFlagLoweredBy(t.state.State, t.instStatus[t.localhost], t.objStatus.ActorStatus != nil && t.isStarted())
	if reason == "" {
		return
	}
	t.log.Infof("clear the stopped flag: %s", reason)
	if err := t.unsetStopped(); err != nil {
		t.log.Errorf("clear the stopped flag: %s", err)
	}
}

// stoppedFlagLoweredBy says what lowers the stopped flag of an instance, and
// "" when nothing does.
//
// An instance that is up was started by someone, who wants it up.
//
// An object that runs all it should, up for a failover object and as many up
// instances as its target for a flex one, is wanted up, and no longer needs
// this instance: the instance missed the request that said so, its node being
// down when the object was started, or it was stopped alone and another took
// over. Left flagged, it would not be a candidate the day one of the
// instances running the object fails.
//
// A flex object short of its target is up too, and that lowers nothing: the
// instance an operator stopped would be the best placed candidate again, and
// start back in the place of the next one, as if the stop had not been asked.
//
// Nothing lowers it on a status that can not tell what the stop left. A stop
// raises the flag before it stops its first resource, and the instance looks
// up until its last one is down: lowered then, the flag let the daemon start
// the instance back once the stop was over. So it stays raised:
//
//   - while an action runs on the instance, as the status is then the one of
//     resources in the middle of it,
//
//   - on a status not updated since the flag was raised, which is the update
//     raising it, or one evaluated before: the stop says it is coming to the
//     daemon as it raises the flag, and the flag can be seen first.
func stoppedFlagLoweredBy(state instance.MonitorState, local instance.Status, isObjectStarted bool) string {
	switch {
	case state.IsDoing():
		return ""
	case !local.UpdatedAt.After(local.StoppedAt):
		return ""
	case local.Avail.Is(status.Up, status.StandbyUp):
		return "the instance is up"
	case isObjectStarted:
		return "the object runs all it should"
	default:
		return ""
	}
}

func (t *Manager) clearStartFailed() {
	if t.state.State != instance.MonitorStateStartFailure {
		return
	}
	if t.objStatus.Avail != status.Up {
		return
	}
	for _, instanceMonitor := range t.instMonitor {
		switch instanceMonitor.GlobalExpect {
		case instance.MonitorGlobalExpectNone:
		default:
			return
		}
	}
	if t.objStatus.Topology == topology.Flex && t.objStatus.Flex.Target > 0 && t.objStatus.UpInstancesCount < t.objStatus.Flex.Target {
		// avoid flex instance start loop
		return
	}
	t.log.Infof("clear instance start failed: the object is up")
	t.transitionTo(instance.MonitorStateIdle)
}

// objectAvail is the avail of the object, and undef while the object status
// is not built yet.
//
// The avail of an object lives behind a pointer the aggregation fills, so
// reading it too early, or on an object that has no actor status at all, is a
// crash rather than an answer.
func (t *Manager) objectAvail() status.T {
	if t.objStatus.ActorStatus == nil {
		return status.Undef
	}
	return t.objStatus.Avail
}
