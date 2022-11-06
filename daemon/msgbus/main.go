package msgbus

import (
	"encoding/json"
	"time"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/util/pubsub"
)

func Pub(bus *pubsub.Bus, v any, labels ...pubsub.Label) {
	bus.Pub(v, labels...)
}

func Sub(bus *pubsub.Bus, name string, v any, labels ...pubsub.Label) pubsub.Subscription {
	announceSub(bus, name)
	return bus.Sub(name, v, labels...)
}

func SubWithTimeout(bus *pubsub.Bus, name string, v any, timeout time.Duration, labels ...pubsub.Label) pubsub.Subscription {
	announceSub(bus, name)
	return bus.SubWithTimeout(name, v, timeout, labels...)
}

func Unsub(bus *pubsub.Bus, sub pubsub.Subscription) {
	name := sub.Stop()
	if name != "" {
		announceUnsub(bus, name)
	}
}

func announceSub(bus *pubsub.Bus, name string) {
	go bus.Pub(event.Event{
		Kind: "event_subscribe",
		ID:   0,
		Time: time.Now(),
		Data: jsonMsg("subscribe name: " + name),
	})
}

func announceUnsub(bus *pubsub.Bus, name string) {
	go bus.Pub(event.Event{
		Kind: "event_unsubscribe",
		ID:   0,
		Time: time.Now(),
		Data: jsonMsg("unsubscribe name: " + name),
	})
}

func jsonMsg(msg string) json.RawMessage {
	return json.RawMessage("\"" + msg + "\"")
}
