package daemonps

import (
	"github.com/google/uuid"

	"opensvc.com/opensvc/util/pubsub"
)

func PubSvcAggDelete(bus *pubsub.Bus, id string, v MonSvcAggDeleted) {
	Pub(bus, NsAgg, pubsub.OpDelete, id, v)
}

func PubSvcAggUpdate(bus *pubsub.Bus, id string, v MonSvcAggUpdated) {
	Pub(bus, NsAgg, pubsub.OpUpdate, id, v)
}

func SubSvcAgg(bus *pubsub.Bus, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(bus, NsAgg, op, name, matching, fn)
}
