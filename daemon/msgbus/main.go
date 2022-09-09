package msgbus

import (
	"time"

	"github.com/google/uuid"

	"opensvc.com/opensvc/core/event"
	ps "opensvc.com/opensvc/util/pubsub"
)

const (
	NsAll = ps.NsAll + iota
	NsEvent
	NsFrozen
	NsFrozenFile
	NsCfgFile
	NsCfg
	NsAgg
	NsNmon
	NsSmon
	NsSetNmon
	NsSetSmon
	NsStatus
)

func Pub(bus *ps.Bus, ns, op uint, id string, i interface{}) {
	publication := ps.Publication{
		Ns:    ns,
		Op:    op,
		Id:    id,
		Value: i,
	}
	bus.Pub(publication)
}

func Sub(bus *ps.Bus, ns, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	subscription := ps.Subscription{
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

func UnSub(bus *ps.Bus, id uuid.UUID) {
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
