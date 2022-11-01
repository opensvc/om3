package msgbus

import (
	"context"
	"time"

	"github.com/google/uuid"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/nodesinfo"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/pubsub"
	"opensvc.com/opensvc/util/san"
)

const (
	NsAll = pubsub.NsAll + iota
	NsEvent
	NsFrozen
	NsFrozenFile
	NsCfgFile
	NsCfg
	NsAgg
	NsNmon
	NsNodeStatus
	NsNodeStatusLabels
	NsNodeStatusPaths
	NsSmon
	NsSetNmon
	NsSetSmon
	NsStatus
	NsHbStatus
	NsHbPing
)

type (
	// Msg wraps any other type so the chan can accept Msg instead of the unrestricted "any"
	Msg any

	Exit struct {
		Path     path.T
		Filename string
	}

	// CfgFileUpdated is emited by a fs watcher when a .conf file is updated or created in etc.
	// The smon goroutine listens to this event and updates the daemondata, which in turns emits a CfgUpdated{} event.
	CfgFileUpdated struct {
		Path     path.T
		Filename string
	}

	// CfgFileRemoved is emited by a fs watcher when a .conf file is removed in etc.
	// The smon goroutine listens to this event and updates the daemondata, which in turns emits a CfgDeleted{} event.
	CfgFileRemoved struct {
		Path     path.T
		Filename string
	}

	// FrozenFileUpdated is emited by a fs watcher when a frozen file is updated or created in var.
	// The nmon goroutine listens to this event and updates the daemondata, which in turns emits a Frozen{} event.
	FrozenFileUpdated struct {
		Path     path.T
		Filename string
	}

	// FrozenFileRemoved is emited by a fs watcher when a frozen file is removed from var.
	// The nmon goroutine listens to this event and updates the daemondata, which in turns emits a Frozen{} event.
	FrozenFileRemoved struct {
		Path     path.T
		Filename string
	}

	CfgDeleted struct {
		Path path.T
		Node string
	}

	CfgUpdated struct {
		Path   path.T
		Node   string
		Config instance.Config
	}

	MonCfgDone struct {
		Path     path.T
		Filename string
	}

	RemoteFileConfig struct {
		Path     path.T
		Node     string
		Filename string
		Updated  time.Time
		Ctx      context.Context
		Err      chan error
	}

	Frozen struct {
		Path  path.T
		Node  string
		Value time.Time
	}

	InstStatusDeleted struct {
		Path path.T
		Node string
	}

	InstStatusUpdated struct {
		Path   path.T
		Node   string
		Status instance.Status
	}

	SetNmon struct {
		Node    string
		Monitor cluster.NodeMonitor
	}

	NodeStatusUpdated struct {
		Node  string
		Value cluster.NodeStatus
	}

	NodeStatusLabelsUpdated struct {
		Node  string
		Value nodesinfo.Labels
	}

	NodeStatusPathsUpdated struct {
		Node  string
		Value san.Paths
	}

	NmonDeleted struct {
		Node string
	}

	NmonUpdated struct {
		Node    string
		Monitor cluster.NodeMonitor
	}

	SetSmon struct {
		Path    path.T
		Node    string
		Monitor instance.Monitor
	}

	SmonDeleted struct {
		Path path.T
		Node string
	}

	SmonUpdated struct {
		Path   path.T
		Node   string
		Status instance.Monitor
	}

	MonSvcAggDeleted struct {
		Path path.T
		Node string
	}

	MonSvcAggUpdated struct {
		Path   path.T
		Node   string
		SvcAgg object.AggregatedStatus
		SrcEv  *Msg
	}

	MonSvcAggDone struct {
		Path path.T
	}

	HbStatusUpdated struct {
		Node   string
		Status cluster.HeartbeatThreadStatus
	}

	HbNodePing struct {
		Node   string
		Status bool
	}
)

func NewMsg(arg any) *Msg {
	var t Msg
	t = arg
	return &t
}

func DropPendingMsg(c <-chan *Msg, duration time.Duration) {
	dropping := make(chan bool)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()
		dropping <- true
		for {
			select {
			case <-c:
			case <-ctx.Done():
				return
			}
		}
	}()
	<-dropping
}

func PubCfgDelete(bus *pubsub.Bus, id string, v CfgDeleted) {
	Pub(bus, NsCfg, pubsub.OpDelete, id, v)
}

func PubCfgUpdate(bus *pubsub.Bus, id string, v CfgUpdated) {
	Pub(bus, NsCfg, pubsub.OpUpdate, id, v)
}

func SubCfg(bus *pubsub.Bus, op uint, name string, matching string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsCfg, op, name, matching, fn)
}

func PubCfgFileRemove(bus *pubsub.Bus, id string, v CfgFileRemoved) {
	Pub(bus, NsCfgFile, pubsub.OpDelete, id, v)
}

func PubCfgFileUpdate(bus *pubsub.Bus, id string, v CfgFileUpdated) {
	Pub(bus, NsCfgFile, pubsub.OpUpdate, id, v)
}

func SubCfgFile(bus *pubsub.Bus, op uint, name string, matching string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsCfgFile, op, name, matching, fn)
}

func PubFrozenFileRemove(bus *pubsub.Bus, id string, v FrozenFileRemoved) {
	Pub(bus, NsFrozenFile, pubsub.OpDelete, id, v)
}

func PubFrozenFileUpdate(bus *pubsub.Bus, id string, v FrozenFileUpdated) {
	Pub(bus, NsFrozenFile, pubsub.OpUpdate, id, v)
}

func SubFrozenFile(bus *pubsub.Bus, op uint, name string, matching string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsFrozenFile, op, name, matching, fn)
}

func SubFrozen(bus *pubsub.Bus, name string, matching string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsFrozen, pubsub.OpUpdate, name, matching, fn)
}

func PubFrozen(bus *pubsub.Bus, id string, v Frozen) {
	Pub(bus, NsFrozen, pubsub.OpUpdate, id, v)
}

func PubInstStatusDelete(bus *pubsub.Bus, id string, v InstStatusDeleted) {
	Pub(bus, NsStatus, pubsub.OpDelete, id, v)
}

func PubInstStatusUpdated(bus *pubsub.Bus, id string, v InstStatusUpdated) {
	Pub(bus, NsStatus, pubsub.OpUpdate, id, v)
}

func SubInstStatus(bus *pubsub.Bus, op uint, name string, matching string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsStatus, op, name, matching, fn)
}

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

func PubSmonDelete(bus *pubsub.Bus, id string, v SmonDeleted) {
	Pub(bus, NsSmon, pubsub.OpDelete, id, v)
}

func PubSmonUpdated(bus *pubsub.Bus, id string, v SmonUpdated) {
	Pub(bus, NsSmon, pubsub.OpUpdate, id, v)
}

func SubSmon(bus *pubsub.Bus, op uint, name string, matching string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsSmon, op, name, matching, fn)
}

func PubSetSmonUpdated(bus *pubsub.Bus, id string, v SetSmon) {
	Pub(bus, NsSetSmon, pubsub.OpUpdate, id, v)
}

func SubSetSmon(bus *pubsub.Bus, op uint, name string, matching string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsSetSmon, op, name, matching, fn)
}

func PubSvcAggDelete(bus *pubsub.Bus, id string, v MonSvcAggDeleted) {
	Pub(bus, NsAgg, pubsub.OpDelete, id, v)
}

func PubSvcAggUpdate(bus *pubsub.Bus, id string, v MonSvcAggUpdated) {
	Pub(bus, NsAgg, pubsub.OpUpdate, id, v)
}

func SubSvcAgg(bus *pubsub.Bus, op uint, name string, matching string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsAgg, op, name, matching, fn)
}

func PubNodeStatusUpdate(bus *pubsub.Bus, v NodeStatusUpdated) {
	Pub(bus, NsNodeStatus, pubsub.OpUpdate, "", v)
}

func SubNodeStatus(bus *pubsub.Bus, op uint, name string, matching string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsNodeStatus, op, name, matching, fn)
}

func PubNodeStatusLabelsUpdate(bus *pubsub.Bus, v NodeStatusLabelsUpdated) {
	Pub(bus, NsNodeStatusLabels, pubsub.OpUpdate, "", v)
}

func SubNodeStatusLabels(bus *pubsub.Bus, op uint, name string, matching string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsNodeStatusLabels, op, name, matching, fn)
}

func PubNodeStatusPathsUpdate(bus *pubsub.Bus, v NodeStatusPathsUpdated) {
	Pub(bus, NsNodeStatusPaths, pubsub.OpUpdate, "", v)
}

func SubNodeStatusPaths(bus *pubsub.Bus, op uint, name string, matching string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsNodeStatusPaths, op, name, matching, fn)
}

func PubHbStatusUpdate(bus *pubsub.Bus, id string, v HbStatusUpdated) {
	Pub(bus, NsHbStatus, pubsub.OpUpdate, id, v)
}

func SubHbStatusUpdate(bus *pubsub.Bus, op uint, name string, matching string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsHbStatus, op, name, matching, fn)
}

func PubHbNodePing(bus *pubsub.Bus, id string, v HbNodePing) {
	Pub(bus, NsHbPing, pubsub.OpUpdate, id, v)
}

func SubHbNodePing(bus *pubsub.Bus, op uint, name string, matching string, fn func(i any)) uuid.UUID {
	return Sub(bus, NsHbPing, op, name, matching, fn)
}
