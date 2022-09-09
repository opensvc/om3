package msgbus

import (
	"github.com/google/uuid"

	"opensvc.com/opensvc/util/pubsub"
)

func PubInstStatusDelete(bus *pubsub.Bus, id string, v InstStatusDeleted) {
	Pub(bus, NsStatus, pubsub.OpDelete, id, v)
}

func PubInstStatusUpdated(bus *pubsub.Bus, id string, v InstStatusUpdated) {
	Pub(bus, NsStatus, pubsub.OpUpdate, id, v)
}

func SubInstStatus(bus *pubsub.Bus, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(bus, NsStatus, op, name, matching, fn)
}
