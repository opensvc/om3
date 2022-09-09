package msgbus

import (
	"time"

	"github.com/google/uuid"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/util/pubsub"
)

func Pub(bus *pubsub.Bus, ns, op uint, id string, i any) {
	publication := pubsub.Publication{
		Ns:    ns,
		Op:    op,
		Id:    id,
		Value: i,
	}
	bus.Pub(publication)
}

func Sub(bus *pubsub.Bus, ns, op uint, name string, matching string, fn func(i any)) uuid.UUID {
	subscription := pubsub.Subscription{
		Ns:       ns,
		Op:       op,
		Matching: matching,
		Name:     name,
	}
	go PubEvent(bus, event.Event{
		Kind: "event_subscribe",
		ID:   0,
		Time: time.Now(),
		Data: jsonMsg("subscribe name: " + name),
	})

	return bus.Sub(subscription, fn)
}

func UnSub(bus *pubsub.Bus, id uuid.UUID) {
	name := bus.Unsub(id)
	if name != "" {
		go PubEvent(bus, event.Event{
			Kind: "event_unsubscribe",
			ID:   0,
			Time: time.Now(),
			Data: jsonMsg("unsubscribe name: " + name),
		})
	}
}
