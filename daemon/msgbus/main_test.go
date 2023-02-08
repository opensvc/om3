package msgbus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/util/pubsub"
)

func TestSubscriptionFilter(t *testing.T) {
	bus := pubsub.NewBus(t.Name())
	bus.Start(context.Background())
	defer bus.Stop()

	sub := bus.Sub(t.Name())
	sub.AddFilter(HbNodePing{}, pubsub.Label{"node", "node10"})
	sub.Start()
	defer sub.Stop()

	// publish non watched type
	bus.Pub(HbStale{}, pubsub.Label{"node", "node1"})

	// publish message with watched type but not watched label
	bus.Pub(HbNodePing{
		Node:   "node1",
		Status: true,
	}, pubsub.Label{"node", "node1"})

	// publish message with watched type but without label
	bus.Pub(HbNodePing{
		Node:   "node1",
		Status: true,
	})

	// publish message with the watched type and label
	bus.Pub(HbNodePing{
		Node:   "node10",
		Status: true,
	}, pubsub.Label{"node", "node10"})

	t.Log("verify received message from correct label")
	i := <-sub.C
	require.Equal(t, "node10", i.(HbNodePing).Node)

	t.Log("ensure no unexpected message")
	select {
	case i := <-sub.C:
		t.Fatalf("unexpected message received %v", i)
	case <-time.After(time.Millisecond):
	}
}
