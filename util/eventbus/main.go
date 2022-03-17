package eventbus

import (
	"encoding/json"

	"github.com/google/uuid"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/util/pubsub"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	T struct {
		pubsub.T
	}
)

func Pub(cmdC chan<- interface{}, e event.Event) {
	pubsub.Pub(cmdC, e)
}

func Sub(cmdC chan<- interface{}, name string, fn func(event.Event)) uuid.UUID {
	f := func(i interface{}) {
		fn(i.(event.Event))
	}
	go Pub(cmdC, event.Event{
		Kind:      "event-subscribe",
		ID:        0,
		Timestamp: timestamp.Now(),
		Data:      jsonMsg(name),
	})
	return pubsub.Sub(cmdC, "subscribe name: "+name, f)
}

func UnSub(cmdC chan<- interface{}, id uuid.UUID) {
	name := pubsub.Unsub(cmdC, id)
	if name != "" {
		go Pub(cmdC, event.Event{
			Kind:      "event-unsubscribe",
			ID:        0,
			Timestamp: timestamp.Now(),
			Data:      jsonMsg("unsubscribe name: " + name),
		})
	}
}

func jsonMsg(msg string) *json.RawMessage {
	d := json.RawMessage("\"" + msg + "\"")
	return &d
}
