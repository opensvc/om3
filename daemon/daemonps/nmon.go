package daemonps

import (
	"github.com/google/uuid"

	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/pubsub"
)

func PubNmonDelete(bus *pubsub.Bus, v moncmd.NmonDeleted) {
	Pub(bus, NsNmon, pubsub.OpDelete, "", v)
}

func PubNmonUpdated(bus *pubsub.Bus, v moncmd.NmonUpdated) {
	Pub(bus, NsNmon, pubsub.OpUpdate, "", v)
}

func SubNmon(bus *pubsub.Bus, op uint, name string, fn func(i interface{})) uuid.UUID {
	return Sub(bus, NsNmon, op, name, "", fn)
}

func PubSetNmonUpdated(bus *pubsub.Bus, v moncmd.SetNmon) {
	Pub(bus, NsSetNmon, pubsub.OpUpdate, "", v)
}

func SubSetNmon(bus *pubsub.Bus, op uint, name string, fn func(i interface{})) uuid.UUID {
	return Sub(bus, NsSetNmon, op, name, "", fn)
}
