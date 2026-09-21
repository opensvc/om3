package nmon

import (
	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// logSetOrchestrationID names in the logs the orchestration this node monitor
// is running, and names none when it is running none.
//
// The logger is swapped rather than added to: a zerolog context only grows, so
// adding the attribute again on every orchestration would leave an idle
// monitor stamping every entry with the last id it saw, or with zeroes.
func (t *Manager) logSetOrchestrationID(i uuid.UUID) {
	if i == uuid.Nil {
		t.log = t.logBase
		return
	}
	t.log = t.logBase.Attr("orchestration_id", i.String())
}

// adoptOrchestration takes on the orchestration the requester was handed an id
// for.
//
// The api mints the id, answers it to the client, and asks for the global
// expect. Until the monitor adopts it the id names nothing: it is not on the
// node monitor the daemons replicate, not on the execs this forks, and not in
// any log entry, so a client polling the id it was given finds nothing ever
// happened. This is the object monitor's onSetInstanceMonitor doing the same.
func (t *Manager) adoptOrchestration(candidate uuid.UUID) {
	if candidate == uuid.Nil || t.state.OrchestrationID == candidate {
		return
	}
	t.abortPendingOrchestration()
	t.logSetOrchestrationID(candidate)
	t.state.OrchestrationID = candidate
	t.publishOrchestrationAccepted()
	t.setNextPendingOrchestration()
}

// endOrchestration is called when the global expect this was running has been
// reached, or given up on.
//
// Which of the two it was is said on the end, from the state the monitor ends
// on: a client waiting on the id is asking how its request went, and an end
// that says nothing about it would have every client read the state back and
// judge for itself.
func (t *Manager) endOrchestration() {
	if t.orchestrationPending != nil && t.state.State.IsOneOf(node.MonitorStatesFailure...) {
		t.orchestrationPending.Failed = true
		t.orchestrationPending.Error = t.state.State.String()
	}
	defer t.publishOrchestrationEnded()
	t.state.OrchestrationID = uuid.Nil
	t.logSetOrchestrationID(uuid.Nil)
}

// expect is the state the orchestration is for. A node is asked to freeze by
// a global expect and to drain by a local one, so a listing that only ever
// read the global one reported a drain as targeting "none".
func (t *Manager) expect() string {
	if t.state.GlobalExpect != node.MonitorGlobalExpectNone &&
		t.state.GlobalExpect != node.MonitorGlobalExpectInit {
		return t.state.GlobalExpect.String()
	}
	if t.state.LocalExpect != node.MonitorLocalExpectNone &&
		t.state.LocalExpect != node.MonitorLocalExpectInit {
		return t.state.LocalExpect.String()
	}
	return ""
}

func (t *Manager) setNextPendingOrchestration() {
	t.orchestrationPending = &msgbus.NodeOrchestrationEnd{
		Msg:                   pubsub.Msg{},
		Node:                  t.localhost,
		ID:                    t.state.OrchestrationID.String(),
		Expect:                t.expect(),
		GlobalExpect:          t.state.GlobalExpect,
		GlobalExpectUpdatedAt: t.state.GlobalExpectUpdatedAt,
	}
}

func (t *Manager) publishOrchestrationAccepted() {
	t.publisher.Pub(&msgbus.NodeOrchestrationAccepted{
		Msg:                   pubsub.Msg{},
		Node:                  t.localhost,
		ID:                    t.state.OrchestrationID.String(),
		Expect:                t.expect(),
		GlobalExpect:          t.state.GlobalExpect,
		GlobalExpectUpdatedAt: t.state.GlobalExpectUpdatedAt,
	}, t.labelLocalhost)
}

// abortPendingOrchestration ends the orchestration a newer one displaces. It
// ended, and aborted is how it ended, so it is said now rather than held until
// the next orchestration ends: a client polling the id it was handed should
// not wait on an orchestration nothing is running any more.
func (t *Manager) abortPendingOrchestration() {
	if t.orchestrationPending == nil {
		return
	}
	t.orchestrationPending.Aborted = true
	t.publisher.Pub(t.orchestrationPending, t.labelLocalhost)
	t.orchestrationPending = nil
}

func (t *Manager) publishOrchestrationEnded() {
	if t.orchestrationPending == nil {
		return
	}
	t.publisher.Pub(t.orchestrationPending, t.labelLocalhost)
	t.orchestrationPending = nil
}

// publishOrchestrationRefused says the global expect was not taken on, so a
// client polling the id it was handed is not left waiting for an orchestration
// that never started.
func (t *Manager) publishOrchestrationRefused(id uuid.UUID, globalExpect *node.MonitorGlobalExpect, reason string) {
	if id == uuid.Nil {
		return
	}
	t.publisher.Pub(&msgbus.NodeOrchestrationRefused{
		Msg:          pubsub.Msg{},
		Node:         t.localhost,
		ID:           id.String(),
		Reason:       reason,
		GlobalExpect: globalExpect,
	}, t.labelLocalhost)
}
