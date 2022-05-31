// Package daemonps define pub-sub namespaces for daemon
package daemonps

import (
	"encoding/json"

	"github.com/google/uuid"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/daemon/busids"
	ps "opensvc.com/opensvc/util/pubsub"
	"opensvc.com/opensvc/util/timestamp"
)

const (
	NsAll = ps.NsAll + iota
	NsEvent
	NsCfgFile
	NsCfg
	NsAgg
	NsSmon
	NsSetSmon
	NsStatus
)

// PubEvent publish a new event.Event on namespace NsEvent
func PubEvent(cmdC chan<- interface{}, e event.Event) {
	ps.Pub(cmdC, ps.Publication{Ns: NsEvent, Op: ps.OpCreate, Value: e})
}

// SubEvent subscribes on namespace NsEvent
func SubEvent(cmdC chan<- interface{}, name string, fn func(event.Event)) uuid.UUID {
	f := func(i interface{}) {
		fn(i.(event.Event))
	}
	publication := ps.Publication{
		Ns: busids.NsEvent,
		Op: ps.OpCreate,
		Id: "subscribe-event",
		Value: event.Event{
			Kind:      "event-subscribe",
			ID:        0,
			Timestamp: timestamp.Now(),
			Data:      jsonMsg("subscribe name: " + name),
		},
	}

	go ps.Pub(cmdC, publication)

	subscription := ps.Subscription{
		Ns:   busids.NsEvent,
		Op:   ps.OpCreate,
		Name: name,
	}
	return ps.Sub(cmdC, subscription, f)
}

// UnSubEvent unsubscribes a subscription on namespace NsEvent
func UnSubEvent(cmdC chan<- interface{}, id uuid.UUID) {
	name := ps.Unsub(cmdC, id)
	if name != "" {
		publication := ps.Publication{
			Ns: busids.NsEvent,
			Op: ps.OpCreate,
			Id: "unsubscribe-event",
			Value: event.Event{
				Kind:      "event-unsubscribe",
				ID:        0,
				Timestamp: timestamp.Now(),
				Data:      jsonMsg("unsubscribe name: " + name),
			},
		}
		go ps.Pub(cmdC, publication)
	}
}

func jsonMsg(msg string) *json.RawMessage {
	d := json.RawMessage("\"" + msg + "\"")
	return &d
}
