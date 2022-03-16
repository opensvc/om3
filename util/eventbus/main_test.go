package eventbus

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/core/event"
)

func TestEventbus(t *testing.T) {
	bus := &T{}
	cmdC, err := bus.Run(context.Background(), t.Name())
	require.Nil(t, err)
	defer bus.Stop()
	names := []string{"hb_stale", "hb_beating"}
	var eventsToPub []event.Event
	var clientEvents1 []event.Event
	var clientEvents2 []event.Event
	for _, evName := range names {
		eventsToPub = append(eventsToPub, event.Event{Kind: evName})
	}
	sub1 := Sub(cmdC, "client1", func(e event.Event) { clientEvents1 = append(clientEvents1, e) })
	sub2 := Sub(cmdC, "client2", func(e event.Event) { clientEvents2 = append(clientEvents2, e) })
	for _, e := range eventsToPub {
		Pub(cmdC, e)
	}
	require.Equal(t, eventsToPub, clientEvents1)
	require.Equal(t, eventsToPub, clientEvents2)
	UnSub(cmdC, sub1)
	UnSub(cmdC, sub2)
}
