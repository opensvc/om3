package imon

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

func newClearTestManager(pub *pubSpy) *Manager {
	m := newTestManager(pub)
	m.orchestrationClearTimer = stoppedTimer()
	return m
}

func ended(pub *pubSpy) *msgbus.ObjectOrchestrationEnd {
	for _, msg := range pub.msgs {
		if v, ok := msg.(*msgbus.ObjectOrchestrationEnd); ok {
			return v
		}
	}
	return nil
}

func refusedWith(pub *pubSpy, id uuid.UUID) *msgbus.ObjectOrchestrationRefused {
	for _, msg := range pub.msgs {
		if v, ok := msg.(*msgbus.ObjectOrchestrationRefused); ok && v.ID == id.String() {
			return v
		}
	}
	return nil
}

func acceptedWith(pub *pubSpy, id uuid.UUID) *msgbus.ObjectOrchestrationAccepted {
	for _, msg := range pub.msgs {
		if v, ok := msg.(*msgbus.ObjectOrchestrationAccepted); ok && v.ID == id.String() {
			return v
		}
	}
	return nil
}

// The end of an orchestration, which ends the wait of its requester, is
// announced once every peer cleared the orchestration: a next request asked
// at once to a peer found it still in progress there, and was refused.
func TestTheEndWaitsForThePeersToClear(t *testing.T) {
	pub := &pubSpy{}
	m := newClearTestManager(pub)
	id := uuid.New()
	m.state.OrchestrationID = id
	m.setNextPendingOrchestration()
	m.instMonitor["node2"] = instance.Monitor{OrchestrationID: id, OrchestrationIsDone: true}
	m.instMonitor["node3"] = instance.Monitor{}

	m.state.OrchestrationID = uuid.Nil
	m.endOrchestrationWhenPeersCleared()
	assert.Nil(t, ended(pub), "node2 still carries the orchestration")

	m.instMonitor["node2"] = instance.Monitor{}
	m.onOrchestrationClearCheck()
	e := ended(pub)
	require.NotNil(t, e, "every peer cleared it")
	assert.Equal(t, id.String(), e.ID)
}

// A peer that never clears, as one gone, does not hold the end beyond the
// timeout.
func TestTheEndIsAnnouncedOnTimeout(t *testing.T) {
	pub := &pubSpy{}
	m := newClearTestManager(pub)
	id := uuid.New()
	m.state.OrchestrationID = id
	m.setNextPendingOrchestration()
	m.instMonitor["node2"] = instance.Monitor{OrchestrationID: id}
	m.state.OrchestrationID = uuid.Nil
	m.endOrchestrationWhenPeersCleared()
	assert.Nil(t, ended(pub))

	m.orchestrationEndingSince = time.Now().Add(-orchestrationClearTimeout)
	m.onOrchestrationClearCheck()
	assert.NotNil(t, ended(pub))
}

// stopRequest asks the stop of the object under a new orchestration id.
func stopRequest() *msgbus.SetInstanceMonitor {
	stopped := instance.MonitorGlobalExpectStopped
	return &msgbus.SetInstanceMonitor{
		Value: instance.MonitorUpdate{GlobalExpect: &stopped, CandidateOrchestrationID: uuid.New()},
	}
}

// doneHere sets the monitor in an orchestration done here, not cleared as a
// peer is not seen done.
func doneHere(m *Manager) {
	m.state.OrchestrationID = uuid.New()
	m.state.GlobalExpect = instance.MonitorGlobalExpectStarted
	m.state.OrchestrationIsDone = true
}

// A request for a new orchestration received while the one before it is
// done here but not cleared is held, then taken on once it clears, as v2
// took a new request once the instances were idle, rather than refused as
// already in progress.
func TestARequestIsHeldUntilTheOrchestrationClears(t *testing.T) {
	pub := &pubSpy{}
	m := newClearTestManager(pub)
	doneHere(m)
	c := stopRequest()
	m.onSetInstanceMonitor(c)
	assert.Nil(t, refusedWith(pub, c.Value.CandidateOrchestrationID), "held, not refused")
	assert.Nil(t, acceptedWith(pub, c.Value.CandidateOrchestrationID))
	assert.Same(t, c, m.heldRequest)

	// The orchestration before clears.
	m.state.OrchestrationID = uuid.Nil
	m.state.GlobalExpect = instance.MonitorGlobalExpectNone
	m.state.OrchestrationIsDone = false
	m.onOrchestrationClearCheck()
	assert.Nil(t, m.heldRequest)
	assert.NotNil(t, acceptedWith(pub, c.Value.CandidateOrchestrationID))
	assert.Equal(t, c.Value.CandidateOrchestrationID, m.state.OrchestrationID)
	assert.Equal(t, instance.MonitorGlobalExpectStopped, m.state.GlobalExpect)
}

// A request held while the orchestration before it does not clear is
// refused as already in progress after the timeout.
func TestARequestHeldIsRefusedOnTimeout(t *testing.T) {
	pub := &pubSpy{}
	m := newClearTestManager(pub)
	doneHere(m)
	c := stopRequest()
	m.onSetInstanceMonitor(c)
	m.heldRequestSince = time.Now().Add(-heldRequestTimeout)
	m.onOrchestrationClearCheck()
	r := refusedWith(pub, c.Value.CandidateOrchestrationID)
	require.NotNil(t, r)
	assert.Contains(t, r.Reason, "already in progress")
	assert.Nil(t, m.heldRequest)
}

// An orchestration not done here refuses a new request at once, as before.
func TestARequestWhileInProgressIsRefused(t *testing.T) {
	pub := &pubSpy{}
	m := newClearTestManager(pub)
	doneHere(m)
	m.state.OrchestrationIsDone = false
	c := stopRequest()
	m.onSetInstanceMonitor(c)
	r := refusedWith(pub, c.Value.CandidateOrchestrationID)
	require.NotNil(t, r)
	assert.Contains(t, r.Reason, "already in progress")
	assert.Nil(t, m.heldRequest)
}

// An abort refuses the request held, and a newer request supersedes it.
func TestARequestHeldIsRefusedByAnAbortOrANewerOne(t *testing.T) {
	pub := &pubSpy{}
	m := newClearTestManager(pub)
	doneHere(m)
	first := stopRequest()
	m.onSetInstanceMonitor(first)
	second := stopRequest()
	m.onSetInstanceMonitor(second)
	r := refusedWith(pub, first.Value.CandidateOrchestrationID)
	require.NotNil(t, r)
	assert.Contains(t, r.Reason, "superseded")
	assert.Same(t, second, m.heldRequest)

	aborted := instance.MonitorGlobalExpectAborted
	m.onSetInstanceMonitor(&msgbus.SetInstanceMonitor{Value: instance.MonitorUpdate{GlobalExpect: &aborted, CandidateOrchestrationID: uuid.New()}})
	r = refusedWith(pub, second.Value.CandidateOrchestrationID)
	require.NotNil(t, r)
	assert.Equal(t, "aborted", r.Reason)
	assert.Nil(t, m.heldRequest)
}

