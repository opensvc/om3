package daemonps

import (
	"github.com/google/uuid"

	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/pubsub"
)

func PubCfgDelete(bus *pubsub.Bus, id string, v moncmd.CfgDeleted) {
	Pub(bus, NsCfg, pubsub.OpDelete, id, v)
}

func PubCfgUpdate(bus *pubsub.Bus, id string, v moncmd.CfgUpdated) {
	Pub(bus, NsCfg, pubsub.OpUpdate, id, v)
}

func SubCfg(bus *pubsub.Bus, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(bus, NsCfg, op, name, matching, fn)
}
