package msgbus

import (
	"github.com/google/uuid"
	"opensvc.com/opensvc/util/pubsub"
)

func SubFrozen(bus *pubsub.Bus, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(bus, NsFrozen, pubsub.OpUpdate, name, matching, fn)
}

func PubFrozen(bus *pubsub.Bus, id string, v Frozen) {
	Pub(bus, NsFrozen, pubsub.OpUpdate, id, v)
}
