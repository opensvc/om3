package eventbus

import (
	"github.com/google/uuid"

	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/util/pubsub"
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
	return pubsub.Sub(cmdC, name, f)
}

func UnSub(cmdC chan<- interface{}, id uuid.UUID) {
	pubsub.Unsub(cmdC, id)
}
