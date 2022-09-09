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

func PubCfgFileRemove(bus *pubsub.Bus, id string, v moncmd.CfgFileRemoved) {
	Pub(bus, NsCfgFile, pubsub.OpDelete, id, v)
}

func PubCfgFileUpdate(bus *pubsub.Bus, id string, v moncmd.CfgFileUpdated) {
	Pub(bus, NsCfgFile, pubsub.OpUpdate, id, v)
}

func SubCfgFile(bus *pubsub.Bus, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(bus, NsCfgFile, op, name, matching, fn)
}

func PubFrozenFileRemove(bus *pubsub.Bus, id string, v moncmd.FrozenFileRemoved) {
	Pub(bus, NsFrozenFile, pubsub.OpDelete, id, v)
}

func PubFrozenFileUpdate(bus *pubsub.Bus, id string, v moncmd.FrozenFileUpdated) {
	Pub(bus, NsFrozenFile, pubsub.OpUpdate, id, v)
}

func SubFrozenFile(bus *pubsub.Bus, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(bus, NsFrozenFile, op, name, matching, fn)
}
