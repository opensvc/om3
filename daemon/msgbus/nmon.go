package msgbus

import (
	"github.com/google/uuid"

	"opensvc.com/opensvc/util/pubsub"
)

func PubNmonDelete(bus *pubsub.Bus, v NmonDeleted) {
	Pub(bus, NsNmon, pubsub.OpDelete, "", v)
}

func PubNmonUpdated(bus *pubsub.Bus, v NmonUpdated) {
	Pub(bus, NsNmon, pubsub.OpUpdate, "", v)
}

func SubNmon(bus *pubsub.Bus, op uint, name string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsNmon, op, name, "", fn)
}

func PubSetNmon(bus *pubsub.Bus, v SetNmon) {
	Pub(bus, NsSetNmon, pubsub.OpUpdate, "", v)
}

func SubSetNmon(bus *pubsub.Bus, name string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsSetNmon, pubsub.OpUpdate, name, "", fn)
}
