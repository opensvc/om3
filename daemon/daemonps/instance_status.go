package daemonps

import (
	"github.com/google/uuid"

	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/pubsub"
)

func PubInstStatusDelete(cmdC chan<- interface{}, id string, v moncmd.InstStatusDeleted) {
	Pub(cmdC, NsStatus, pubsub.OpDelete, id, v)
}

func PubInstStatusUpdated(cmdC chan<- interface{}, id string, v moncmd.InstStatusUpdated) {
	Pub(cmdC, NsStatus, pubsub.OpUpdate, id, v)
}

func SubInstStatus(cmdC chan<- interface{}, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(cmdC, NsStatus, op, name, matching, fn)
}
