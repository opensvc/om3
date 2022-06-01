package daemonps

import (
	"github.com/google/uuid"

	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/pubsub"
)

func PubCfgDelete(cmdC chan<- interface{}, id string, v moncmd.CfgDeleted) {
	Pub(cmdC, NsCfg, pubsub.OpDelete, id, v)
}

func PubCfgUpdate(cmdC chan<- interface{}, id string, v moncmd.CfgUpdated) {
	Pub(cmdC, NsCfg, pubsub.OpUpdate, id, v)
}

func SubCfg(cmdC chan<- interface{}, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(cmdC, NsCfg, op, name, matching, fn)
}
