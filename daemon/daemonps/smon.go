package daemonps

import (
	"github.com/google/uuid"

	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/pubsub"
)

func PubSmonDelete(cmdC chan<- interface{}, id string, v moncmd.SmonDeleted) {
	Pub(cmdC, NsSmon, pubsub.OpDelete, id, v)
}

func PubSmonUpdated(cmdC chan<- interface{}, id string, v moncmd.SmonUpdated) {
	Pub(cmdC, NsSmon, pubsub.OpUpdate, id, v)
}

func SubSmon(cmdC chan<- interface{}, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(cmdC, NsSmon, op, name, matching, fn)
}

func PubSetSmonUpdated(cmdC chan<- interface{}, id string, v moncmd.SetSmon) {
	Pub(cmdC, NsSetSmon, pubsub.OpUpdate, id, v)
}

func SubSetSmon(cmdC chan<- interface{}, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(cmdC, NsSetSmon, op, name, matching, fn)
}
