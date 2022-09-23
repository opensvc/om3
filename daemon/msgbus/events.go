// Package msgbus define pub-sub namespaces for daemon
package msgbus

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/util/pubsub"
)

// PubEvent publish a new event.Event on namespace NsEvent
func PubEvent(bus *pubsub.Bus, e event.Event) {
	publication := pubsub.Publication{
		Ns:    NsEvent,
		Op:    pubsub.OpCreate,
		Value: e,
	}
	bus.Pub(publication)
}

// SubEvent subscribes on namespace NsEvent
func SubEvent(bus *pubsub.Bus, name string, fn func(event.Event)) uuid.UUID {
	subscription := pubsub.Subscription{
		Ns:   NsEvent,
		Op:   pubsub.OpCreate,
		Name: name,
	}
	return subEvent(bus, subscription, fn)
}

// SubEventWithTimeout subscribes on namespace NsEvent, the subscription will
// be automatically killed if subscriber callback duration exceeds timeout
func SubEventWithTimeout(bus *pubsub.Bus, name string, fn func(event.Event), timeout time.Duration) uuid.UUID {
	subscription := pubsub.Subscription{
		Ns:      NsEvent,
		Op:      pubsub.OpCreate,
		Name:    name,
		Timeout: timeout,
	}
	return subEvent(bus, subscription, fn)
}

// UnSubEvent unsubscribes a subscription on namespace NsEvent
func UnSubEvent(bus *pubsub.Bus, id uuid.UUID) {
	name := bus.Unsub(id)
	if name != "" {
		publication := pubsub.Publication{
			Ns: NsEvent,
			Op: pubsub.OpCreate,
			Id: "unsubscribe-event",
			Value: event.Event{
				Kind: "event_unsubscribe",
				ID:   0,
				Time: time.Now(),
				Data: jsonMsg("unsubscribe name: " + name),
			},
		}
		go bus.Pub(publication)
	}
}

// subEvent subscribes
func subEvent(bus *pubsub.Bus, subscription pubsub.Subscription, fn func(event.Event)) uuid.UUID {
	f := func(i any) {
		if i == nil {
			// happens after pubsub queue is closed (on Unsub)
			return
		}
		fn(i.(event.Event))
	}
	publication := pubsub.Publication{
		Ns: NsEvent,
		Op: pubsub.OpCreate,
		Id: "subscribe-event",
		Value: event.Event{
			Kind: "event_subscribe",
			ID:   0,
			Time: time.Now(),
			Data: jsonMsg("subscribe name: " + subscription.Name),
		},
	}

	go bus.Pub(publication)

	return bus.Sub(subscription, f)
}

func jsonMsg(msg string) json.RawMessage {
	return json.RawMessage("\"" + msg + "\"")
}
