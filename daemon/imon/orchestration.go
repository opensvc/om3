package imon

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/msgbus"
)

func (o *imon) isDone() bool {
	select {
	case <-o.ctx.Done():
		return true
	default:
		return false
	}
}

// orchestrate from omon vs global expect
func (o *imon) orchestrate() {
	if o.isDone() {
		o.log.Debug().Msgf("orchestrate return on isDone()")
		return
	}
	if _, ok := o.instStatus[o.localhost]; !ok {
		o.log.Debug().Msgf("orchestrate return on no instStatus[o.localhost]")
		return
	}
	if _, ok := o.nodeStatus[o.localhost]; !ok {
		o.log.Debug().Msgf("orchestrate return on no nodeStatus[o.localhost]")
		return
	}
	if !o.isConvergedGlobalExpect() {
		o.log.Debug().Msgf("orchestrate return on not isConvergedGlobalExpect")
		return
	}

	switch o.state.GlobalExpect {
	case instance.MonitorGlobalExpectAborted:
		o.orchestrateAborted()
	}

	if o.state.OrchestrationId != uuid.Nil && o.state.OrchestrationIsDone {
		if o.orchestrationIsAllDone() {
			o.endOrchestration()
		}
		o.log.Debug().Msgf("orchestrate return on o.state.OrchestrationId != uuid.Nil && o.state.OrchestrationIsDone")
		return
	}
	if o.isDone() {
		o.log.Debug().Msgf("orchestrate return on isDone()")
		return
	}
	if nodeMonitor, ok := o.nodeMonitor[o.localhost]; !ok {
		o.log.Debug().Msgf("orchestrate return on no nodeMonitor localhost")
		return
	} else if nodeMonitor.State != node.MonitorStateIdle {
		o.log.Debug().Msgf("orchestrate return on nodeMonitor.State != node.MonitorStateIdle")
		return
	}

	o.orchestrateResourceRestart()
	if o.isDone() {
		o.log.Debug().Msgf("orchestrate return on isDone()")
		return
	}

	switch o.state.GlobalExpect {
	case instance.MonitorGlobalExpectDeleted:
		o.orchestrateDeleted()
	case instance.MonitorGlobalExpectNone:
		o.orchestrateNone()
	case instance.MonitorGlobalExpectFrozen:
		o.orchestrateFrozen()
	case instance.MonitorGlobalExpectProvisioned:
		o.orchestrateProvisioned()
	case instance.MonitorGlobalExpectPlaced:
		o.orchestratePlaced()
	case instance.MonitorGlobalExpectPlacedAt:
		o.orchestratePlacedAt()
	case instance.MonitorGlobalExpectPurged:
		o.orchestratePurged()
	case instance.MonitorGlobalExpectStarted:
		o.orchestrateStarted()
	case instance.MonitorGlobalExpectStopped:
		o.orchestrateStopped()
	case instance.MonitorGlobalExpectThawed:
		o.orchestrateThawed()
	case instance.MonitorGlobalExpectUnprovisioned:
		o.orchestrateUnprovisioned()
	}
	o.updateIfChange()
}

func (o *imon) setWaitParents() bool {
	for relation, availStatus := range o.state.Parents {
		if !availStatus.Is(status.Up, status.Undef) {
			if o.state.State != instance.MonitorStateWaitParents {
				o.log.Info().Msgf("wait parents because %s avail status is %s", relation, availStatus)
				o.state.State = instance.MonitorStateWaitParents
				o.change = true
			}
			return true
		}
	}
	if o.state.State == instance.MonitorStateWaitParents {
		o.log.Info().Msgf("stop waiting parents")
		o.state.State = instance.MonitorStateIdle
		o.change = true
	}
	return false
}

func (o *imon) setWaitChildren() bool {
	for relation, availStatus := range o.state.Children {
		if !availStatus.Is(status.Down, status.StandbyDown, status.StandbyUp, status.Undef) {
			if o.state.State != instance.MonitorStateWaitChildren {
				o.log.Info().Msgf("wait children because %s avail status is %s", relation, availStatus)
				o.state.State = instance.MonitorStateWaitChildren
				o.change = true
			}
			return true
		}
	}
	if o.state.State == instance.MonitorStateWaitChildren {
		o.log.Info().Msgf("stop waiting children")
		o.state.State = instance.MonitorStateIdle
		o.change = true
	}
	return false
}

// endOrchestration is called when orchestration has been reached on all nodes
func (o *imon) endOrchestration() {
	o.change = true
	o.state.GlobalExpect = instance.MonitorGlobalExpectNone
	o.state.GlobalExpectOptions = nil
	o.state.OrchestrationIsDone = false
	o.state.OrchestrationId = uuid.UUID{}
	o.clearPending()
	o.updateIfChange()
	if o.acceptedOrchestrationId != uuid.Nil {
		o.pubsubBus.Pub(&msgbus.ObjectOrchestrationEnd{
			Node: o.localhost,
			Path: o.path,
			Id:   o.acceptedOrchestrationId.String(),
		},
			o.labelPath,
			o.labelLocalhost,
		)
		o.acceptedOrchestrationId = uuid.UUID{}
	}
}

// doneAndIdle marks the orchestration as done on the local instance and
// sets the state to idle.
func (o *imon) doneAndIdle() {
	o.done()
	o.state.State = instance.MonitorStateIdle
}

// done() sets marks the orchestration as done on the local instance.
// It can be used instead of doneAndIdle() when we want a state to linger
// after the orchestration is ended.
func (o *imon) done() {
	o.change = true
	o.state.OrchestrationIsDone = true
}

func (o *imon) orchestrationIsAllDone() bool {
	for nodename, oImon := range o.AllInstanceMonitors() {
		if !oImon.OrchestrationIsDone && oImon.OrchestrationId != uuid.Nil {
			msg := fmt.Sprintf("state:%s orchestrationId:%s", oImon.State, oImon.OrchestrationId)
			if o.waitConvergedOrchestrationMsg[nodename] != msg {
				o.log.Info().Msgf("orchestration progress on node %s: %s", nodename, msg)
				o.waitConvergedOrchestrationMsg[nodename] = msg
			}
			return false
		} else {
			// OrchestrationIsDone or no OrchestrationId
			msg := fmt.Sprintf("state:%s orchestrationId:%s", oImon.State, oImon.OrchestrationId)
			if o.waitConvergedOrchestrationMsg[nodename] != msg {
				o.log.Info().Msgf("orchestration done on node %s: %s", nodename, msg)
				o.waitConvergedOrchestrationMsg[nodename] = msg
			}
		}
	}
	if len(o.waitConvergedOrchestrationMsg) > 0 {
		o.log.Info().Msgf("orchestration is done on all nodes")
		o.waitConvergedOrchestrationMsg = make(map[string]string)
	}
	return true
}
