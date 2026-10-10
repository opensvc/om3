package imon

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/placement"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type pubSpy struct {
	msgs []pubsub.Messager
}

func (t *pubSpy) Pub(m pubsub.Messager, _ ...pubsub.Label) {
	t.msgs = append(t.msgs, m)
}

func newTestManager(pub pubsub.Publisher) *Manager {
	return &Manager{
		log:         plog.NewDefaultLogger(),
		publisher:   pub,
		path:        naming.Path{Kind: naming.KindSvc, Name: "obj"},
		localhost:   "node1",
		delayTimer:  time.NewTimer(time.Hour),
		instMonitor: make(map[string]instance.Monitor),
		instStatus:  make(map[string]instance.Status),
		state: instance.Monitor{
			State:        instance.MonitorStateIdle,
			GlobalExpect: instance.MonitorGlobalExpectNone,
			LocalExpect:  instance.MonitorLocalExpectNone,
		},
		// The avail of an object lives behind the pointer the aggregation
		// fills, and the rules a request is judged by read it.
		objStatus: object.Status{
			ActorStatus: &object.ActorStatus{
				Avail:       status.Down,
				Topology:    topology.Failover,
				Provisioned: provisioned.True,
			},
		},
	}
}

// The id a requester is handed names the orchestration its request starts,
// and a request the monitor refuses starts none.
//
// This used to be decided by whether anything had changed the monitor in the
// same pass, which is not the same thing: a refused request that had changed
// something was answered with its error and took the id all the same. The
// monitor then named an orchestration that never ran, with no global expect
// to reach and so nothing to end it, and every later request was refused as
// "already in progress" until an abort cleared it.
func TestARefusedRequestTakesNoOrchestrationID(t *testing.T) {
	pub := &pubSpy{}
	m := newTestManager(pub)

	state := instance.MonitorStateStartFailure
	invalid := instance.MonitorGlobalExpect(-1)
	id := uuid.New()
	m.onSetInstanceMonitor(&msgbus.SetInstanceMonitor{
		Path: m.path,
		Node: m.localhost,
		Value: instance.MonitorUpdate{
			// changes the monitor
			State: &state,
			// and is refused
			GlobalExpect:             &invalid,
			CandidateOrchestrationID: id,
		},
	})

	assert.Equal(t, uuid.Nil, m.state.OrchestrationID,
		"a refused request leaves the monitor naming no orchestration")
	assert.Equal(t, instance.MonitorGlobalExpectNone, m.state.GlobalExpect)

	var refused *msgbus.ObjectOrchestrationRefused
	for _, msg := range pub.msgs {
		if v, ok := msg.(*msgbus.ObjectOrchestrationRefused); ok {
			refused = v
		}
	}
	require.NotNil(t, refused, "the id is answered with the refusal, so it names something")
	assert.Equal(t, id.String(), refused.ID)
}

func TestAnAcceptedRequestTakesTheOrchestrationID(t *testing.T) {
	pub := &pubSpy{}
	m := newTestManager(pub)

	started := instance.MonitorGlobalExpectStarted
	id := uuid.New()
	m.onSetInstanceMonitor(&msgbus.SetInstanceMonitor{
		Path: m.path,
		Node: m.localhost,
		Value: instance.MonitorUpdate{
			GlobalExpect:             &started,
			CandidateOrchestrationID: id,
		},
	})

	assert.Equal(t, id, m.state.OrchestrationID)
	assert.Equal(t, instance.MonitorGlobalExpectStarted, m.state.GlobalExpect)

	var accepted *msgbus.ObjectOrchestrationAccepted
	for _, msg := range pub.msgs {
		if v, ok := msg.(*msgbus.ObjectOrchestrationAccepted); ok {
			accepted = v
		}
	}
	require.NotNil(t, accepted)
	assert.Equal(t, id.String(), accepted.ID)
}

// A request that asks for what the monitor already has changes nothing, and
// is refused rather than answered with an id naming nothing.
func TestARequestChangingNothingIsRefused(t *testing.T) {
	pub := &pubSpy{}
	m := newTestManager(pub)

	none := instance.MonitorLocalExpectNone
	id := uuid.New()
	m.onSetInstanceMonitor(&msgbus.SetInstanceMonitor{
		Path: m.path,
		Node: m.localhost,
		Value: instance.MonitorUpdate{
			LocalExpect:              &none,
			CandidateOrchestrationID: id,
		},
	})

	assert.Equal(t, uuid.Nil, m.state.OrchestrationID)
	var refused *msgbus.ObjectOrchestrationRefused
	for _, msg := range pub.msgs {
		if v, ok := msg.(*msgbus.ObjectOrchestrationRefused); ok {
			refused = v
		}
	}
	require.NotNil(t, refused)
	assert.Equal(t, id.String(), refused.ID)
}

// A start someone asks for is accepted as soon as one instance is provisioned,
// the start candidates being the provisioned instances only. It is refused
// when none is: the orchestration would have no instance to start.
func TestARequestedStartNeedsAProvisionedInstance(t *testing.T) {
	m := newTestManager(&pubSpy{})
	m.scopeNodes = []string{"node1", "node2"}
	m.objStatus.Provisioned = provisioned.Mixed
	m.instStatus["node1"] = instance.Status{Avail: status.Down, Provisioned: provisioned.False}
	m.instStatus["node2"] = instance.Status{Avail: status.Down, Provisioned: provisioned.True}
	ok, reason := m.isRequestedStartable()
	assert.True(t, ok, reason)

	m.instStatus["node2"] = instance.Status{Avail: status.Down, Provisioned: provisioned.Mixed}
	ok, reason = m.isRequestedStartable()
	assert.False(t, ok)
	assert.Contains(t, reason, "no instance provisioned")

	m.objStatus.Provisioned = provisioned.False
	ok, reason = m.isRequestedStartable()
	assert.False(t, ok)
	assert.Contains(t, reason, "false object provisioned state")
}

// A switch moves the object to an instance not up and provisioned: an
// unprovisioned instance is no destination.
func TestASwitchDestinationIsProvisioned(t *testing.T) {
	m := newTestManager(&pubSpy{})
	m.scopeNodes = []string{"node1", "node2", "node3"}
	m.objStatus.PlacementPolicy = placement.NodesOrder
	m.instStatus["node1"] = instance.Status{Avail: status.Up, Provisioned: provisioned.True}
	m.instStatus["node2"] = instance.Status{Avail: status.Down, Provisioned: provisioned.False}
	m.instStatus["node3"] = instance.Status{Avail: status.Down, Provisioned: provisioned.False}
	assert.Equal(t, "", m.nextPlacedAtCandidate(), "no instance not up is provisioned")

	m.instStatus["node3"] = instance.Status{Avail: status.Down, Provisioned: provisioned.True}
	assert.Equal(t, "node3", m.nextPlacedAtCandidate())
}
