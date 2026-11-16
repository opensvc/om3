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

// A request accepted by a node whose clock is behind the node that accepted
// the request before is stamped after it: the peers adopt it, rather than
// keep the request before, which the node adopted back, its request
// accepted and never run.
func TestARequestIsStampedAfterTheOneBefore(t *testing.T) {
	pub := &pubSpy{}
	m := newClearTestManager(pub)
	ahead := time.Now().Add(30 * time.Second)
	m.state.GlobalExpectUpdatedAt = ahead
	m.instMonitor["node2"] = instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectNone, GlobalExpectUpdatedAt: ahead}
	c := stopRequest()
	m.onSetInstanceMonitor(c)
	require.NotNil(t, acceptedWith(pub, c.Value.CandidateOrchestrationID))
	assert.True(t, m.state.GlobalExpectUpdatedAt.After(ahead), "stamped after the request before")

	// A peer update carrying the request before does not take it back.
	m.convergeGlobalExpectFromRemote()
	assert.Equal(t, instance.MonitorGlobalExpectStopped, m.state.GlobalExpect)
	assert.Equal(t, c.Value.CandidateOrchestrationID, m.state.OrchestrationID)
}

// Two nodes accepting a request at once after the same stamp stamp them the
// same: both keep the one of the greater orchestration id. An orchestration
// ended here, its stamp kept and its id cleared, is not adopted back from a
// peer not done clearing it.
func TestEqualStampsConvergeOnTheGreaterID(t *testing.T) {
	stamp := time.Now()
	low := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	high := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	m := newClearTestManager(&pubSpy{})
	m.state.GlobalExpect = instance.MonitorGlobalExpectStarted
	m.state.GlobalExpectUpdatedAt = stamp
	m.state.OrchestrationID = low
	m.instMonitor["node2"] = instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectStopped, GlobalExpectUpdatedAt: stamp, OrchestrationID: high}
	m.convergeGlobalExpectFromRemote()
	assert.Equal(t, high, m.state.OrchestrationID, "the greater id is kept")
	assert.Equal(t, instance.MonitorGlobalExpectStopped, m.state.GlobalExpect)

	m = newClearTestManager(&pubSpy{})
	m.state.GlobalExpect = instance.MonitorGlobalExpectNone
	m.state.GlobalExpectUpdatedAt = stamp
	m.instMonitor["node2"] = instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectStarted, GlobalExpectUpdatedAt: stamp, OrchestrationID: high}
	m.convergeGlobalExpectFromRemote()
	assert.Equal(t, uuid.Nil, m.state.OrchestrationID, "ended here, not adopted back")
	assert.Equal(t, instance.MonitorGlobalExpectNone, m.state.GlobalExpect)
}

// The orchestration accepted here, displaced by the one of a peer accepted
// at once and stamped the same with a greater id, ends aborted rather than
// end as the one adopted does, under its id.
func TestAnOrchestrationDisplacedByConvergenceEndsAborted(t *testing.T) {
	pub := &pubSpy{}
	m := newClearTestManager(pub)
	stamp := time.Now()
	low := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	high := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	m.state.GlobalExpect = instance.MonitorGlobalExpectStarted
	m.state.GlobalExpectUpdatedAt = stamp
	m.state.OrchestrationID = low
	m.setNextPendingOrchestration()
	m.instMonitor["node2"] = instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectStopped, GlobalExpectUpdatedAt: stamp, OrchestrationID: high}
	m.convergeGlobalExpectFromRemote()
	e := ended(pub)
	require.NotNil(t, e)
	assert.Equal(t, low.String(), e.ID)
	assert.True(t, e.Aborted)
	assert.Nil(t, m.orchestrationPending, "the one adopted ends on the node that accepted it")
}

// A node not having a request yet adopts it from the peer running it, not
// the end of it from a peer that ended it, both stamped the same.
func TestALaggingNodeAdoptsTheRequestRunning(t *testing.T) {
	stamp := time.Now()
	id := uuid.New()
	for range 20 {
		m := newClearTestManager(&pubSpy{})
		m.state.GlobalExpectUpdatedAt = stamp.Add(-time.Minute)
		m.instMonitor["node2"] = instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectNone, GlobalExpectUpdatedAt: stamp}
		m.instMonitor["node3"] = instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectStopped, GlobalExpectUpdatedAt: stamp, OrchestrationID: id}
		m.convergeGlobalExpectFromRemote()
		assert.Equal(t, instance.MonitorGlobalExpectStopped, m.state.GlobalExpect)
		assert.Equal(t, id, m.state.OrchestrationID)
	}
}
