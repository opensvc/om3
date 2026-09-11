package nmon

import (
	"testing"

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
