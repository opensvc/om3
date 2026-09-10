package session

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
	"github.com/opensvc/om3/v3/util/xsession"
)

func sid(u uuid.UUID) xsession.Id { return xsession.NewSid(u) }
func oid(u uuid.UUID) xsession.Id { return xsession.NewOid(u) }
func eid(u uuid.UUID) xsession.Id { return xsession.NewEid(u) }

// An exec and its outcome make one session, and the object it acts on is read
// from the label the message carries it in.
func TestAnExecAndItsOutcomeMakeOneSession(t *testing.T) {
	reset()
	m := &Manager{}
	id := uuid.New()

	m.handle(&msgbus.Exec{
		Msg:       pubsub.Msg{Labels: pubsub.Labels{"path": "test/svc/s1"}},
		Command:   "om test/svc/s1 instance start",
		Node:      "n1",
		Origin:    "api",
		SessionID: sid(id),
		ExecID:    eid(id),
	})
	s, ok := firstSession(id.String())
	require.True(t, ok)
	assert.Equal(t, StateRunning, s.State)
	assert.Equal(t, "test/svc/s1", s.Path, "the path comes from the label")
	assert.Equal(t, "api", s.Origin)

	m.handle(&msgbus.ExecSuccess{SessionID: sid(id), ExecID: eid(id), Duration: 2 * time.Second})
	s, _ = firstSession(id.String())
	assert.Equal(t, StateSucceeded, s.State)
	assert.Equal(t, 2*time.Second, s.Duration)
}

func TestAFailedExecKeepsWhatFailed(t *testing.T) {
	reset()
	m := &Manager{}
	id := uuid.New()

	m.handle(&msgbus.Exec{SessionID: sid(id), ExecID: eid(id)})
	m.handle(&msgbus.ExecFailed{SessionID: sid(id), ExecID: eid(id), ErrS: "exit code 1", Duration: time.Second})

	s, _ := firstSession(id.String())
	assert.Equal(t, StateFailed, s.State)
	assert.Equal(t, "exit code 1", s.Error)
}

// A session run under an orchestration is found by that orchestration's id,
// which is how a client that submitted an orchestrated action follows it.
func TestASessionOfAnOrchestrationIsFoundByIt(t *testing.T) {
	reset()
	m := &Manager{}
	orchestrationID := uuid.New()
	first, second := uuid.New(), uuid.New()

	m.handle(&msgbus.ObjectOrchestrationAccepted{
		ID:   orchestrationID.String(),
		Node: "n1",
		Path: naming.Path{Name: "s1", Kind: naming.KindSvc},
	})
	m.handle(&msgbus.Exec{SessionID: sid(first), ExecID: eid(first), OrchestrationID: oid(orchestrationID)})
	m.handle(&msgbus.Exec{SessionID: sid(second), ExecID: eid(second), OrchestrationID: oid(orchestrationID)})
	m.handle(&msgbus.Exec{SessionID: sid(uuid.New())})

	l := ListSessions(Filter{OrchestrationID: orchestrationID.String()})
	assert.Len(t, l, 2, "the sessions of the orchestration, and no other")

	o, ok := GetOrchestration(orchestrationID.String())
	require.True(t, ok)
	assert.Equal(t, StateRunning, o.State)

	m.handle(&msgbus.ObjectOrchestrationEnd{ID: orchestrationID.String()})
	o, _ = GetOrchestration(orchestrationID.String())
	assert.Equal(t, StateSucceeded, o.State)
}

// An action that is a step of nothing says so. The id an exec carries for its
// own logs is a fresh one when there is no orchestration, and reporting it
// would claim membership of an orchestration that never existed.
func TestAnExecOutsideAnOrchestrationClaimsNone(t *testing.T) {
	reset()
	m := &Manager{}
	id := uuid.New()

	m.handle(&msgbus.Exec{SessionID: sid(id), ExecID: eid(id)})

	s, _ := firstSession(id.String())
	assert.Equal(t, "", s.OrchestrationID)
	assert.Len(t, ListSessions(Filter{OrchestrationID: uuid.New().String()}), 0)
}

func TestAnAbortedAndARefusedOrchestration(t *testing.T) {
	reset()
	m := &Manager{}
	aborted, refused := uuid.New(), uuid.New()

	m.handle(&msgbus.ObjectOrchestrationAccepted{ID: aborted.String()})
	m.handle(&msgbus.ObjectOrchestrationEnd{ID: aborted.String(), Aborted: true})
	o, _ := GetOrchestration(aborted.String())
	assert.Equal(t, StateAborted, o.State)

	m.handle(&msgbus.ObjectOrchestrationRefused{ID: refused.String(), Reason: "node is frozen"})
	o, _ = GetOrchestration(refused.String())
	assert.Equal(t, StateRefused, o.State)
	assert.Equal(t, "node is frozen", o.Error)
}

// Every node holds the instance monitor of every node, so a node that
// accepted nothing can still answer for an orchestration: the monitors
// carrying its id are what says it is running, and their dropping it is what
// says it is over.
func TestAnOrchestrationIsKnownFromTheMonitorsAlone(t *testing.T) {
	reset()
	m := &Manager{}
	id := uuid.New()
	at := time.Now().Add(-time.Minute)

	mon := func(node string, oid uuid.UUID) *msgbus.InstanceMonitorUpdated {
		return &msgbus.InstanceMonitorUpdated{
			Path: naming.Path{Name: "s1", Kind: naming.KindSvc},
			Node: node,
			Value: instance.Monitor{
				OrchestrationID:       oid,
				GlobalExpect:          instance.MonitorGlobalExpectStarted,
				GlobalExpectUpdatedAt: at,
			},
		}
	}

	m.handle(mon("n1", id))
	m.handle(mon("n2", id))

	o, ok := GetOrchestration(id.String())
	require.True(t, ok, "no node accepted it here, and it is known all the same")
	assert.Equal(t, StateRunning, o.State)
	assert.Equal(t, "s1", o.Path)
	assert.Equal(t, at, o.BeginAt, "the start is the one every node agrees on")

	// One node reaching it is not the end of it.
	m.handle(mon("n1", uuid.Nil))
	o, _ = GetOrchestration(id.String())
	assert.Equal(t, StateRunning, o.State, "a node still carries it")

	// The last one is.
	m.handle(mon("n2", uuid.Nil))
	o, _ = GetOrchestration(id.String())
	assert.Equal(t, StateSucceeded, o.State)
	require.NotNil(t, o.EndAt)
}

// A monitor naming no orchestration must not invent one.
func TestAMonitorWithoutAnOrchestrationRecordsNone(t *testing.T) {
	reset()
	m := &Manager{}
	m.handle(&msgbus.InstanceMonitorUpdated{
		Path:  naming.Path{Name: "s1", Kind: naming.KindSvc},
		Node:  "n1",
		Value: instance.Monitor{OrchestrationID: uuid.Nil},
	})
	assert.Len(t, ListOrchestrations(Filter{}), 0)
}

// How an orchestration ended is known only where the outcome was published,
// and the monitors falling silent must not overwrite it with a success.
func TestTheMonitorsDoNotOverwriteAKnownOutcome(t *testing.T) {
	reset()
	m := &Manager{}
	id := uuid.New()
	mon := func(oid uuid.UUID) *msgbus.InstanceMonitorUpdated {
		return &msgbus.InstanceMonitorUpdated{
			Path:  naming.Path{Name: "s1", Kind: naming.KindSvc},
			Node:  "n1",
			Value: instance.Monitor{OrchestrationID: oid},
		}
	}
	m.handle(mon(id))
	m.handle(&msgbus.ObjectOrchestrationEnd{ID: id.String(), Aborted: true})
	m.handle(mon(uuid.Nil))

	o, _ := GetOrchestration(id.String())
	assert.Equal(t, StateAborted, o.State, "aborted, not succeeded")
}

// The node of an orchestration is the one that accepted it, which only the
// acceptance says. Every node of the object names the id in its monitor, so
// naming one from a monitor would make the field mean whichever node this
// daemon happened to hear from first.
func TestOnlyTheAcceptanceNamesTheAcceptingNode(t *testing.T) {
	reset()
	m := &Manager{}
	id := uuid.New()

	m.handle(&msgbus.InstanceMonitorUpdated{
		Path:  naming.Path{Name: "s1", Kind: naming.KindSvc},
		Node:  "n2",
		Value: instance.Monitor{OrchestrationID: id},
	})
	o, _ := GetOrchestration(id.String())
	assert.Equal(t, "", o.Node, "a monitor says the node is in it, not that it accepted it")

	m.handle(&msgbus.ObjectOrchestrationAccepted{
		ID:   id.String(),
		Node: "n1",
		Path: naming.Path{Name: "s1", Kind: naming.KindSvc},
	})
	o, _ = GetOrchestration(id.String())
	assert.Equal(t, "n1", o.Node)
}
