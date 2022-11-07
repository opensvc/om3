package msgbus

import (
	"encoding/json"
	"time"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/util/pubsub"
)

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
