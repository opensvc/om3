package nmon

import (
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type recordingPublisher struct {
	messages []pubsub.Messager
}

func (p *recordingPublisher) Pub(message pubsub.Messager, _ ...pubsub.Label) {
	p.messages = append(p.messages, message)
}

func TestUpdateIsOverloadedAndPublish(t *testing.T) {
	const nodename = "node1"
	publisher := &recordingPublisher{}
	manager := &Manager{
		localhost: nodename,
		log:       plog.NewDefaultLogger(),
		nodeConfig: node.Config{
			MinAvailMemPct:  2,
			MinAvailSwapPct: 0,
		},
		nodeStatus: node.Status{IsOverloaded: true},
		publisher:  publisher,
	}
	t.Cleanup(func() { node.StatusData.Unset(nodename) })

	manager.updateIsOverloadedAndPublish(node.Stats{
		MemAvailPct: 80,
		MemTotalMB:  4096,
	})

	if manager.nodeStatus.IsOverloaded {
		t.Fatal("expected overload status to be cleared")
	}
	var statusUpdates []*msgbus.NodeStatusUpdated
	for _, message := range publisher.messages {
		if update, ok := message.(*msgbus.NodeStatusUpdated); ok {
			statusUpdates = append(statusUpdates, update)
		}
	}
	if len(statusUpdates) != 1 {
		t.Fatalf("got %d node status updates, want 1", len(statusUpdates))
	}
	if statusUpdates[0].Value.IsOverloaded {
		t.Fatal("published node status still reports overload")
	}

	manager.nodeConfig.MinAvailSwapPct = 10
	manager.updateIsOverloadedAndPublish(node.Stats{
		MemAvailPct: 80,
		MemTotalMB:  4096,
	})
	statusUpdates = statusUpdates[:0]
	for _, message := range publisher.messages {
		if update, ok := message.(*msgbus.NodeStatusUpdated); ok {
			statusUpdates = append(statusUpdates, update)
		}
	}
	if len(statusUpdates) != 2 {
		t.Fatalf("got %d node status updates after enabling the swap threshold, want 2", len(statusUpdates))
	}
	if !statusUpdates[1].Value.IsOverloaded {
		t.Fatal("published node status does not report overload after enabling the swap threshold")
	}

	publishedCount := len(publisher.messages)
	manager.updateIsOverloadedAndPublish(node.Stats{
		MemAvailPct: 80,
		MemTotalMB:  4096,
	})
	if len(publisher.messages) != publishedCount {
		t.Fatal("unchanged overload status was published again")
	}
}

// newTestManager is a manager with just enough in it to drive the
// orchestration lifecycle: the state, the logger it swaps, and somewhere to
// record what it publishes.
func newTestManager(publisher *recordingPublisher) *Manager {
	m := &Manager{
		localhost: "node1",
		logBase:   plog.NewDefaultLogger(),
		publisher: publisher,
		state:     node.Monitor{State: node.MonitorStateIdle},
	}
	m.logSetOrchestrationID(uuid.Nil)
	return m
}

func kinds(messages []pubsub.Messager) []string {
	l := make([]string, len(messages))
	for i, m := range messages {
		if k, ok := m.(interface{ Kind() string }); ok {
			l[i] = k.Kind()
		} else {
			l[i] = fmt.Sprintf("%T", m)
		}
	}
	return l
}

// A drain is asked for with a local expect, not a global one, so it ends in
// orchestrateDrained rather than where a freeze ends. It has to end all the
// same: an orchestration adopted and never ended stays running in the store
// for as long as the store keeps it, and stamps every later log entry of an
// idle monitor with its id.
func TestADrainEndsItsOrchestration(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state node.MonitorState
	}{
		{"drained", node.MonitorStateDrainSuccess},
		{"failed to drain", node.MonitorStateDrainFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			publisher := &recordingPublisher{}
			m := newTestManager(publisher)
			id := uuid.New()

			m.state.LocalExpect = node.MonitorLocalExpectDrained
			m.adoptOrchestration(id)
			if m.state.OrchestrationID != id {
				t.Fatalf("the orchestration was not adopted: %s", m.state.OrchestrationID)
			}

			m.state.State = tc.state
			m.orchestrateDrained()

			if m.state.OrchestrationID != uuid.Nil {
				t.Errorf("the orchestration was not ended: %s", m.state.OrchestrationID)
			}
			if m.state.LocalExpect != node.MonitorLocalExpectNone {
				t.Errorf("the local expect was not cleared: %s", m.state.LocalExpect)
			}
			want := []string{"NodeOrchestrationAccepted", "NodeOrchestrationEnd"}
			got := kinds(publisher.messages)
			if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
				t.Errorf("published %v, want %v", got, want)
			}
		})
	}
}

// The id a second request is handed displaces the first, which ended all the
// same, and aborted is how it ended.
func TestANewOrchestrationAbortsTheOneItDisplaces(t *testing.T) {
	publisher := &recordingPublisher{}
	m := newTestManager(publisher)

	m.adoptOrchestration(uuid.New())
	m.adoptOrchestration(uuid.New())

	want := []string{"NodeOrchestrationAccepted", "NodeOrchestrationEnd", "NodeOrchestrationAccepted"}
	got := kinds(publisher.messages)
	if len(got) != len(want) {
		t.Fatalf("published %v, want %v", got, want)
	}
	end, ok := publisher.messages[1].(*msgbus.NodeOrchestrationEnd)
	if !ok || !end.Aborted {
		t.Errorf("the displaced orchestration did not end aborted: %#v", publisher.messages[1])
	}
}
