package daemonps

import (
	"github.com/google/uuid"

	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/pubsub"
)

func PubSvcAggDelete(cmdC chan<- interface{}, id string, v moncmd.MonSvcAggDeleted) {
	Pub(cmdC, NsAgg, pubsub.OpDelete, id, v)
}

func PubSvcAggUpdate(cmdC chan<- interface{}, id string, v moncmd.MonSvcAggUpdated) {
	Pub(cmdC, NsAgg, pubsub.OpUpdate, id, v)
}

func SubSvcAgg(cmdC chan<- interface{}, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(cmdC, NsAgg, op, name, matching, fn)
}
