package daemonps

import (
	"github.com/google/uuid"

	"opensvc.com/opensvc/util/pubsub"
)

func PubCfgDelete(bus *pubsub.Bus, id string, v CfgDeleted) {
	Pub(bus, NsCfg, pubsub.OpDelete, id, v)
}

func PubCfgUpdate(bus *pubsub.Bus, id string, v CfgUpdated) {
	Pub(bus, NsCfg, pubsub.OpUpdate, id, v)
}

func SubCfg(bus *pubsub.Bus, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(bus, NsCfg, op, name, matching, fn)
}

func PubCfgFileRemove(bus *pubsub.Bus, id string, v CfgFileRemoved) {
	Pub(bus, NsCfgFile, pubsub.OpDelete, id, v)
}

func PubCfgFileUpdate(bus *pubsub.Bus, id string, v CfgFileUpdated) {
	Pub(bus, NsCfgFile, pubsub.OpUpdate, id, v)
}

func SubCfgFile(bus *pubsub.Bus, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(bus, NsCfgFile, op, name, matching, fn)
}

func PubFrozenFileRemove(bus *pubsub.Bus, id string, v FrozenFileRemoved) {
	Pub(bus, NsFrozenFile, pubsub.OpDelete, id, v)
}

func PubFrozenFileUpdate(bus *pubsub.Bus, id string, v FrozenFileUpdated) {
	Pub(bus, NsFrozenFile, pubsub.OpUpdate, id, v)
}

func SubFrozenFile(bus *pubsub.Bus, op uint, name string, matching string, fn func(i interface{})) uuid.UUID {
	return Sub(bus, NsFrozenFile, op, name, matching, fn)
}
