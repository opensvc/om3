package imon

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/msgbus"
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
		t.log.Debugf("orchestrate return on isDone()")
		return
	}
	if _, ok := t.instStatus[t.localhost]; !ok {
		t.log.Debugf("orchestrate return on no instStatus[o.localhost]")
		return
	}
	if _, ok := t.nodeStatus[t.localhost]; !ok {
		t.log.Debugf("orchestrate return on no nodeStatus[o.localhost]")
		return
	}
	if !t.isConvergedGlobalExpect() {
		t.log.Debugf("orchestrate return on not isConvergedGlobalExpect")
		return
	}

	switch t.state.GlobalExpect {
	case instance.MonitorGlobalExpectAborted:
		t.orchestrateAborted()
	}

	if t.state.OrchestrationID != uuid.Nil && t.state.OrchestrationIsDone {
		if t.orchestrationIsAllDone() {
			t.endOrchestration()
		}
		t.log.Debugf("orchestrate return on o.state.OrchestrationID != uuid.Nil && o.state.OrchestrationIsDone")
		return
	}
	if t.isDone() {
		t.log.Debugf("orchestrate return on isDone()")
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
		t.log.Debugf("orchestrate return on nodeMonitor.State: %s", t.nodeMonitor[t.localhost].State)
		return
	}

	t.orchestrateResourceRestart()
	if t.isDone() {
		t.log.Debugf("orchestrate return on isDone()")
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
	globalExpect := t.state.GlobalExpect
	globalExpectUpdatedAt := t.state.GlobalExpectUpdatedAt
	globalExpectOptions := t.state.GlobalExpectOptions

	t.state.GlobalExpect = instance.MonitorGlobalExpectNone
	t.state.GlobalExpectOptions = nil
	t.state.OrchestrationIsDone = false
	t.state.OrchestrationID = uuid.UUID{}
	t.clearPending()
	t.updateIfChange()
	if t.acceptedOrchestrationID != uuid.Nil {
		t.pub.Pub(&msgbus.ObjectOrchestrationEnd{
			Node:                  t.localhost,
			Path:                  t.path,
			ID:                    t.acceptedOrchestrationID.String(),
			GlobalExpect:          globalExpect,
			GlobalExpectUpdatedAt: globalExpectUpdatedAt,
			GlobalExpectOptions:   globalExpectOptions,
		}, t.pubLabels...)
		t.acceptedOrchestrationID = uuid.UUID{}
	}
	t.log = t.newLogger(uuid.Nil)
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
		t.log.Debugf("set OrchestrationIsDone -> true for OrchestrationID %s", t.state.OrchestrationID)
		t.change = true
		t.state.OrchestrationIsDone = true
	} else if !t.state.OrchestrationIsDone {
		t.log.Debugf("skip change OrchestrationIsDone (OrchestrationID is nil)")
	}
}

func (t *Manager) orchestrationIsAllDone() bool {
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
