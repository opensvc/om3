package msgbus

import (
	"github.com/google/uuid"

	"opensvc.com/opensvc/util/pubsub"
)

func PubSmonDelete(bus *pubsub.Bus, id string, v SmonDeleted) {
	Pub(bus, NsSmon, pubsub.OpDelete, id, v)
}

func PubSmonUpdated(bus *pubsub.Bus, id string, v SmonUpdated) {
	Pub(bus, NsSmon, pubsub.OpUpdate, id, v)
}

func SubSmon(bus *pubsub.Bus, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(bus, NsSmon, op, name, matching, fn)
}

func PubSetSmonUpdated(bus *pubsub.Bus, id string, v SetSmon) {
	Pub(bus, NsSetSmon, pubsub.OpUpdate, id, v)
}

func SubSetSmon(bus *pubsub.Bus, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(bus, NsSetSmon, op, name, matching, fn)
}
