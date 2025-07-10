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
	bus.SetPanicOnFullQueue(time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = pubsub.ContextWithBus(ctx, bus)
	bus.Start(ctx)
	defer bus.Stop()

	sub := pubsub.SubFromContext(ctx, t.Name())
	sub.AddFilter(&NodeAlive{}, pubsub.Label{"node", "node10"})
	sub.Start()
	defer sub.Stop()

	pub := pubsub.PubFromContext(ctx)

	// publish non watched type
	pub.Pub(&HeartbeatStale{}, pubsub.Label{"node", "node1"})

	// publish message with watched type but not watched label
	pub.Pub(&NodeAlive{
		Node: "node1",
	}, pubsub.Label{"node", "node1"})

	// publish message with watched type but without label
	pub.Pub(&NodeAlive{
		Node: "node1",
	})

	// publish message with the watched type and label
	pub.Pub(&NodeAlive{
		Node: "node10",
	}, pubsub.Label{"node", "node10"})

	receiveMsgTimeout := 50 * time.Millisecond
	t.Logf("verify received message from correct label (timeout: %s)", receiveMsgTimeout)
	select {
	case i := <-sub.C:
		require.Equal(t, "node10", i.(*NodeAlive).Node)
	case <-time.After(receiveMsgTimeout):
		t.Fatalf("timeout, no message received")
	}

	t.Log("ensure no unexpected message")
	select {
	case i := <-sub.C:
		t.Fatalf("unexpected message received %v", i)
	case <-time.After(time.Millisecond):
	}
}
