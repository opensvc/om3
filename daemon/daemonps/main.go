package daemonps

import (
	"github.com/google/uuid"

	"opensvc.com/opensvc/core/event"
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

func Pub(cmdC chan<- interface{}, ns, op uint, id string, i interface{}) {
	publication := ps.Publication{
		Ns:    ns,
		Op:    op,
		Id:    id,
		Value: i,
	}
	ps.Pub(cmdC, publication)
}

func Sub(cmdC chan<- interface{}, ns, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	subscription := ps.Subscription{
		Ns:       ns,
		Op:       op,
		Matching: matching,
		Name:     name,
	}
	go PubEvent(cmdC, event.Event{
		Kind:      "event_subscribe",
		ID:        0,
		Timestamp: timestamp.Now(),
		Data:      jsonMsg("subscribe name: " + name),
	})

	return ps.Sub(cmdC, subscription, fn)
}

func UnSub(cmdC chan<- interface{}, id uuid.UUID) {
	name := ps.Unsub(cmdC, id)
	if name != "" {
		go PubEvent(cmdC, event.Event{
			Kind:      "event_unsubscribe",
			ID:        0,
			Timestamp: timestamp.Now(),
			Data:      jsonMsg("unsubscribe name: " + name),
		})
	}
}
