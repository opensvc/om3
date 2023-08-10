package imon

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
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
		return
	}
	if _, ok := o.instStatus[o.localhost]; !ok {
		return
	}
	if _, ok := o.nodeStatus[o.localhost]; !ok {
		return
	}
	if !o.isConvergedGlobalExpect() {
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
		return
	}
	if o.isDone() {
		return
	}
	if nodeMonitor, ok := o.nodeMonitor[o.localhost]; !ok {
		return
	} else if nodeMonitor.State != node.MonitorStateIdle {
		return
	}

	o.orchestrateResourceRestart()
	if o.isDone() {
		return
	}

	switch o.state.GlobalExpect {
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

// endOrchestration is called when orchestration has been reached on all nodes
func (o *imon) endOrchestration() {
	o.change = true
	o.state.State = instance.MonitorStateIdle
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

// doneAndIdle set state to reached and unset orchestration id, it is used to
// set convergence for orchestration reached expectation on all instances.
func (o *imon) doneAndIdle() {
	o.change = true
	o.state.State = instance.MonitorStateIdle
	o.state.OrchestrationIsDone = true
}

func (o *imon) done() {
	o.change = true
	o.state.OrchestrationIsDone = true
}

func (o *imon) orchestrationIsAllDone() bool {
	for nodename, oImon := range o.instMonitor {
		if !oImon.OrchestrationIsDone {
			msg := fmt.Sprintf("state:%s orchestrationId:%s", oImon.State, oImon.OrchestrationId)
			if o.waitConvergedOrchestrationMsg[nodename] != msg {
				o.log.Info().Msgf("orchestration progress on node %s: %s", nodename, msg)
				o.waitConvergedOrchestrationMsg[nodename] = msg
			}
			return false
		} else {
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
