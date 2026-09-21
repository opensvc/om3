package imon

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/status"
)

func (t *Manager) isDone() bool {
	select {
	case <-t.ctx.Done():
		return true
	default:
		return false
	}
}

// orchestrate from omon vs global expect
func (t *Manager) orchestrate() {
	if t.isDone() {
		t.log.Tracef("orchestrate return on isDone()")
		return
	}
	if _, ok := t.instStatus[t.localhost]; !ok {
		t.log.Tracef("orchestrate return on no instStatus[o.localhost]")
		return
	}
	if _, ok := t.nodeStatus[t.localhost]; !ok {
		t.log.Tracef("orchestrate return on no nodeStatus[o.localhost]")
		return
	}
	if !t.isConvergedGlobalExpect() {
		t.log.Tracef("orchestrate return on not isConvergedGlobalExpect")
		return
	}

	switch t.state.GlobalExpect {
	case instance.MonitorGlobalExpectAborted:
		t.orchestrateAborted()
	}

	if t.state.OrchestrationID != uuid.Nil && t.state.OrchestrationIsDone && !t.statusQueued.Load() {
		if t.orchestrationIsDoneOnAll() {
			t.endOrchestration()
		}
		t.log.Tracef("orchestrate return on o.state.OrchestrationID != uuid.Nil && o.state.OrchestrationIsDone")
		return
	}
	if t.isDone() {
		t.log.Tracef("orchestrate return on isDone()")
		return
	}
	switch t.nodeMonitor[t.localhost].State {
	case node.MonitorStateIdle:
		// default orchestrate
	case node.MonitorStateShutdownProgress:
		// accept only local expect shutdown orchestration
		switch t.state.LocalExpect {
		case instance.MonitorLocalExpectShutdown:
			t.orchestrateLocalExpectShutdown()
		}
		return
	default:
		t.log.Tracef("orchestrate return on nodeMonitor.State: %s", t.nodeMonitor[t.localhost].State)
		return
	}

	if t.statusQueued.Load() {
		// a new orchestrate() call will be fired by the InstanceStatusUpdated at the end of the running status evaluation
		t.log.Tracef("orchestrate return on t.statusQueued")
		return
	}

	t.clearStoppedFlagOnRequest()

	t.orchestrateResourceRestart()
	if t.isDone() {
		t.log.Tracef("orchestrate return on isDone()")
		return
	}

	switch t.state.GlobalExpect {
	case instance.MonitorGlobalExpectDeleted:
		t.orchestrateDeleted()
	case instance.MonitorGlobalExpectNone:
		t.orchestrateNone()
	case instance.MonitorGlobalExpectFrozen:
		t.orchestrateFrozen()
	case instance.MonitorGlobalExpectProvisioned:
		t.orchestrateProvisioned()
	case instance.MonitorGlobalExpectPlaced:
		t.orchestratePlaced()
	case instance.MonitorGlobalExpectPlacedAt:
		t.orchestratePlacedAt()
	case instance.MonitorGlobalExpectPurged:
		t.orchestratePurged()
	case instance.MonitorGlobalExpectResized:
		t.orchestrateResized()
	case instance.MonitorGlobalExpectRestarted:
		t.orchestrateRestarted()
	case instance.MonitorGlobalExpectStarted:
		t.orchestrateStarted()
	case instance.MonitorGlobalExpectStopped:
		t.orchestrateStopped()
	case instance.MonitorGlobalExpectUnfrozen:
		t.orchestrateUnfrozen()
	case instance.MonitorGlobalExpectUnprovisioned:
		t.orchestrateUnprovisioned()
	}
	t.updateIfChange()
}

func (t *Manager) isAnyParentWaitingChilren() bool {
	l := make([]string, 0)
	for relation, _ := range t.state.Parents {
		path, err := naming.ParsePath(relation)
		if err != nil {
			t.log.Errorf("%s", err)
			return false
		}
		for nodename, instMon := range instance.MonitorData.GetByPath(path) {
			if instMon.State == instance.MonitorStateWaitChildren {
				l = append(l, fmt.Sprintf("%s@%s", path, nodename))
			}
		}
	}
	if len(l) > 0 {
		t.log.Infof("instances of parents are still waiting for children: %s", strings.Join(l, ","))
		return true
	}
	return false
}

func (t *Manager) setWaitParents() bool {
	for relation, availStatus := range t.state.Parents {
		if !availStatus.Is(status.Up, status.Undef) {
			if t.state.State != instance.MonitorStateWaitParents {
				t.log.Infof("wait parents because %s avail status is %s", relation, availStatus)
				t.state.State = instance.MonitorStateWaitParents
				t.change = true
			}
			return true
		}
	}
	if t.state.State == instance.MonitorStateWaitParents {
		t.log.Infof("stop waiting parents")
		t.state.State = instance.MonitorStateIdle
		t.change = true
	}
	return false
}

// setWaitChildren set or reset wait children, return true is state is wait children
func (t *Manager) setWaitChildren() bool {
	for relation, availStatus := range t.state.Children {
		if !availStatus.Is(status.Down, status.StandbyDown, status.StandbyUp, status.Undef, status.NotApplicable) {
			if t.state.State != instance.MonitorStateWaitChildren {
				t.log.Infof("wait children because %s avail status is %s", relation, availStatus)
				t.state.State = instance.MonitorStateWaitChildren
				t.change = true
			}
			return true
		}
	}
	if t.state.State == instance.MonitorStateWaitChildren {
		t.log.Infof("no more children to wait")
		t.state.State = instance.MonitorStateIdle
		t.change = true
	}
	return false
}

// endOrchestration is called when orchestration has been reached on all nodes
func (t *Manager) endOrchestration() {
	t.change = true

	t.setOrchestrationVerdict()
	defer t.publishOrchestrationEnded()

	t.clearStateSpokenToPeers()
	t.state.GlobalExpect = instance.MonitorGlobalExpectNone
	t.state.GlobalExpectOptions = nil
	t.state.OrchestrationIsDone = false
	t.state.OrchestrationID = uuid.UUID{}
	t.clearPending()
	t.updateIfChange()
	t.logSetOrchestrationID(uuid.Nil)
}

// setOrchestrationVerdict says on the end of the orchestration whether it did
// what it was for.
//
// An orchestration ends when every node is done with it, and a node is done
// with it whether it reached what was asked or gave up on it. The states the
// instances ended on are what tells the two apart, and the accepting node
// holds them all, so the verdict is made here rather than left to every
// client to make again from the states it can read.
func (t *Manager) setOrchestrationVerdict() {
	if t.orchestrationPending == nil {
		return
	}
	failures := make([]string, 0)
	for _, nodename := range t.scopeNodes {
		instMon, ok := t.AllInstanceMonitors()[nodename]
		if !ok {
			continue
		}
		if instMon.State.IsOneOf(instance.MonitorStatesFailure...) {
			failures = append(failures, fmt.Sprintf("%s on %s", instMon.State, nodename))
		}
	}
	if len(failures) == 0 {
		return
	}
	t.orchestrationPending.Failed = true
	t.orchestrationPending.Error = strings.Join(failures, ", ")
}

// clearStateSpokenToPeers puts the instance back to idle when the state it
// ends on was there to be read by the peers rather than by an operator.
//
// A resize walks a chain in stages, and a node that has finished says so in
// its state, because a peer only grows a replicated link once every node has
// grown what is under it. Every node is done by the time this runs, so there
// is nobody left to tell, and an instance holding the size it was asked for
// is not something to keep reporting: the size is in the status. A failure
// stays, as the state of every failed action does.
func (t *Manager) clearStateSpokenToPeers() {
	if t.state.State == instance.MonitorStateResizeSuccess {
		t.state.State = instance.MonitorStateIdle
	}
}

// doneAndIdle marks the orchestration as done on the local instance and
// sets the state to idle.
func (t *Manager) doneAndIdle() {
	t.done()
	if t.state.State != instance.MonitorStateIdle {
		t.change = true
		t.state.State = instance.MonitorStateIdle
	}
}

// done() sets marks the orchestration as done on the local instance.
// It can be used instead of doneAndIdle() when we want a state to linger
// after the orchestration is ended.
// OrchestrationIsDone is set to true when orchestrationID is set.
func (t *Manager) done() {
	if t.state.OrchestrationID != uuid.Nil && !t.state.OrchestrationIsDone {
		t.log.Tracef("set OrchestrationIsDone -> true for OrchestrationID %s", t.state.OrchestrationID)
		t.change = true
		t.state.OrchestrationIsDone = true
	} else if !t.state.OrchestrationIsDone {
		t.log.Tracef("skip change OrchestrationIsDone (OrchestrationID is nil)")
	}
}

func (t *Manager) orchestrationIsDoneOnAll() bool {
	for nodename, oImon := range t.AllInstanceMonitors() {
		if !oImon.OrchestrationIsDone && oImon.OrchestrationID != uuid.Nil {
			msg := fmt.Sprintf("state:%s orchestrationID:%s", oImon.State, oImon.OrchestrationID)
			if t.waitConvergedOrchestrationMsg[nodename] != msg {
				t.log.Infof("orchestration progress on node %s: %s", nodename, msg)
				t.waitConvergedOrchestrationMsg[nodename] = msg
			}
			return false
		} else {
			// OrchestrationIsDone or no OrchestrationID
			msg := fmt.Sprintf("state:%s orchestrationID:%s", oImon.State, oImon.OrchestrationID)
			if t.waitConvergedOrchestrationMsg[nodename] != msg {
				t.log.Infof("orchestration done on node %s: %s", nodename, msg)
				t.waitConvergedOrchestrationMsg[nodename] = msg
			}
		}
	}
	if len(t.waitConvergedOrchestrationMsg) > 0 {
		t.log.Infof("orchestration is done on all nodes")
		t.waitConvergedOrchestrationMsg = make(map[string]string)
	}
	return true
}

func (t *Manager) orchestrationIsDoneOnPeers() bool {
	for nodename, oImon := range t.AllInstanceMonitors() {
		if nodename == t.localhost {
			continue
		}
		if !oImon.OrchestrationIsDone && oImon.OrchestrationID != uuid.Nil {
			msg := fmt.Sprintf("state:%s orchestrationID:%s", oImon.State, oImon.OrchestrationID)
			if t.waitConvergedOrchestrationMsg[nodename] != msg {
				t.log.Infof("orchestration progress on node %s: %s", nodename, msg)
				t.waitConvergedOrchestrationMsg[nodename] = msg
			}
			return false
		} else {
			// OrchestrationIsDone or no OrchestrationID
			msg := fmt.Sprintf("state:%s orchestrationID:%s", oImon.State, oImon.OrchestrationID)
			if t.waitConvergedOrchestrationMsg[nodename] != msg {
				t.log.Infof("orchestration done on node %s: %s", nodename, msg)
				t.waitConvergedOrchestrationMsg[nodename] = msg
			}
		}
	}
	if len(t.waitConvergedOrchestrationMsg) > 0 {
		t.log.Infof("orchestration is done on all nodes")
		t.waitConvergedOrchestrationMsg = make(map[string]string)
	}
	return true
}

// clearStoppedFlagOnRequest lowers the stopped flag when a user asked for the
// object to be up.
//
// The flag is lowered on every instance in the scope, including the ones that
// stay down, because the request is about the object: an instance left
// flagged would not be a candidate the next time the object has to move.
func (t *Manager) clearStoppedFlagOnRequest() {
	switch t.state.GlobalExpect {
	case instance.MonitorGlobalExpectStarted,
		instance.MonitorGlobalExpectRestarted,
		instance.MonitorGlobalExpectPlaced,
		instance.MonitorGlobalExpectPlacedAt:
	default:
		return
	}
	if !t.isStopped() {
		return
	}
	t.log.Infof("clear the stopped flag: the object is wanted up")
	if err := t.unsetStopped(); err != nil {
		t.log.Errorf("clear the stopped flag: %s", err)
	}
}
