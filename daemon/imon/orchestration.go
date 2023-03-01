package imon

import (
	"fmt"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
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
	if err := o.orchestrationRateLimiter.Wait(o.ctx); err != nil {
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

	if o.state.State == instance.MonitorStateReached {
		if !o.isConvergedOrchestrationReached() {
			return
		}
		o.endOrchestration()
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
	o.log.Info().Msgf("leave reached global expect: %s", o.state.GlobalExpect)
	o.change = true
	o.state.State = instance.MonitorStateIdle
	o.state.GlobalExpect = instance.MonitorGlobalExpectNone
	o.state.GlobalExpectOptions = nil
	o.clearPending()
	o.updateIfChange()
	if o.acceptedOrchestrationId != "" {
		o.pubsubBus.Pub(msgbus.ObjectOrchestrationEnd{
			Node:  o.localhost,
			Path:  o.path,
			Id:    o.acceptedOrchestrationId,
			Error: nil,
		},
			pubsub.Label{"path", o.path.String()},
			pubsub.Label{"node", o.localhost},
		)
		o.acceptedOrchestrationId = ""
	}
}

// setReached set state to reached and unset orchestration id, it is used to
// set convergence for orchestration reached expectation on all instances.
func (o *imon) setReached() {
	o.change = true
	o.state.State = instance.MonitorStateReached
	o.state.OrchestrationId = ""
}

// isConvergedOrchestrationReached returns true instance orchestration is reached
// for all object instances (OrchestrationId == "" for all instance monitor cache).
func (o *imon) isConvergedOrchestrationReached() bool {
	for nodename, oImon := range o.instMonitor {
		if oImon.OrchestrationId != "" {
			msg := fmt.Sprintf("state:%s orchestrationId:%s", oImon.State, oImon.OrchestrationId)
			if o.waitConvergedOrchestrationMsg[nodename] != msg {
				o.log.Info().Msgf("not yet converged orchestration (node: %s %s)", nodename, msg)
				o.waitConvergedOrchestrationMsg[nodename] = msg
			}
			return false
		}
	}
	if len(o.waitConvergedOrchestrationMsg) > 0 {
		o.log.Info().Msgf("converged orchestration")
		o.waitConvergedOrchestrationMsg = make(map[string]string)
	}
	return true
}
