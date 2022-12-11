package msgbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/nodesinfo"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/san"
)

type (
	ApiClient struct {
		Time time.Time
		Name string
	}

	CfgDeleted struct {
		Path path.T
		Node string
	}

	// CfgFileRemoved is emitted by a fs watcher when a .conf file is removed in etc.
	// The smon goroutine listens to this event and updates the daemondata, which in turns emits a CfgDeleted{} event.
	CfgFileRemoved struct {
		Path     path.T
		Filename string
	}

	// CfgFileUpdated is emitted by a fs watcher when a .conf file is updated or created in etc.
	// The smon goroutine listens to this event and updates the daemondata, which in turns emits a CfgUpdated{} event.
	CfgFileUpdated struct {
		Path     path.T
		Filename string
	}

	CfgUpdated struct {
		Path   path.T
		Node   string
		Config instance.Config
	}

	ClientSub struct {
		ApiClient
	}

	ClientUnSub struct {
		ApiClient
	}

	// DataUpdated is a patch of changed data
	DataUpdated struct {
		json.RawMessage
	}

	DaemonCtl struct {
		Component string
		Action    string
	}

	Exit struct {
		Path     path.T
		Filename string
	}

	Frozen struct {
		Path  path.T
		Node  string
		Value time.Time
	}

	// FrozenFileRemoved is emitted by a fs watcher when a frozen file is removed from var.
	// The nmon goroutine listens to this event and updates the daemondata, which in turns emits a Frozen{} event.
	FrozenFileRemoved struct {
		Path     path.T
		Filename string
	}

	// FrozenFileUpdated is emitted by a fs watcher when a frozen file is updated or created in var.
	// The nmon goroutine listens to this event and updates the daemondata, which in turns emits a Frozen{} event.
	FrozenFileUpdated struct {
		Path     path.T
		Filename string
	}

	HbNodePing struct {
		Node   string
		Status bool
	}

	HbPing struct {
		Nodename string
		HbId     string
		Time     time.Time
	}

	HbStale struct {
		Nodename string
		HbId     string
		Time     time.Time
	}

	HbStatusUpdated struct {
		Node   string
		Status cluster.HeartbeatThreadStatus
	}

	InstanceMonitorDeleted struct {
		Path path.T
		Node string
	}

	InstanceMonitorUpdated struct {
		Path   path.T
		Node   string
		Status instance.Monitor
	}

	InstanceStatusDeleted struct {
		Path path.T
		Node string
	}

	InstanceStatusUpdated struct {
		Path   path.T
		Node   string
		Status instance.Status
	}

	MonCfgDone struct {
		Path     path.T
		Filename string
	}

	NodeMonitorDeleted struct {
		Node string
	}

	NodeMonitorUpdated struct {
		Node    string
		Monitor cluster.NodeMonitor
	}

	NodeOsPathsUpdated struct {
		Node  string
		Value san.Paths
	}

	NodeStatusLabelsUpdated struct {
		Node  string
		Value nodesinfo.Labels
	}

	NodeStatusUpdated struct {
		Node  string
		Value cluster.NodeStatus
	}

	ObjectAggDeleted struct {
		Path path.T
		Node string
	}

	ObjectAggDone struct {
		Path path.T
	}

	ObjectAggUpdated struct {
		Path             path.T
		Node             string
		AggregatedStatus object.AggregatedStatus
		SrcEv            any
	}

	RemoteFileConfig struct {
		Path     path.T
		Node     string
		Filename string
		Updated  time.Time
		Ctx      context.Context
		Err      chan error
	}

	SetInstanceMonitor struct {
		Path    path.T
		Node    string
		Monitor instance.Monitor
	}

	SetNodeMonitor struct {
		Node    string
		Monitor cluster.NodeMonitor
	}
)

const (
	CfgDeletedAsName              = "deleted object config"
	CfgFileRemovedAsName          = "deleted object config file"
	CfgFileUpdatedAsName          = "updated object config file"
	CfgUpdatedAsName              = "updated object config"
	ClientSubAsName               = "subscribe"
	ClientUnSubAsName             = "unsubscribe"
	DataUpdatedAsName             = "data updated"
	DaemonCtlAsName               = "daemon component action"
	ExitAsName                    = "ExitAsEvent"
	FrozenAsName                  = "updated frozen"
	FrozenFileRemovedAsName       = "deleted frozen file"
	FrozenFileUpdatedAsName       = "updated frozen file"
	HbNodePingAsName              = "updated node ping"
	HbPingAsName                  = "hb node ping"
	HbStaleAsName                 = "hb node stale"
	HbStatusUpdatedAsName         = "updated hb status"
	InstanceMonitorDeletedAsName  = "deleted instance monitor"
	InstanceMonitorUpdatedAsName  = "updated instance monitor"
	InstanceStatusDeletedAsName   = "deleted instance status"
	InstanceStatusUpdatedAsName   = "updated instance status"
	MonCfgDoneAsName              = "done monitor config"
	NodeMonitorDeletedAsName      = "deleted node monitor"
	NodeMonitorUpdatedAsName      = "updated node monitor"
	NodeOsPathsUpdatedAsName      = "updated node os paths"
	NodeStatusLabelsUpdatedAsName = "updated node label"
	NodeStatusUpdatedAsName       = "updated node status"
	ObjectAggDeletedAsName        = "deleted object aggregated status"
	ObjectAggDoneAsName           = "done object aggregated status"
	ObjectAggUpdatedAsName        = "updated object aggregated status"
	RemoteFileConfigAsName        = "updated remote config file"
	SetInstanceMonitorAsName      = "set instance monitor"
	SetNodeMonitorAsName          = "set node monitor"
)

func DropPendingMsg(c <-chan any, duration time.Duration) {
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

func (e ApiClient) Bytes() []byte {
	return []byte(fmt.Sprintf("%s %s", e.Name, e.Time))
}

func (e CfgDeleted) Event() string {
	return CfgDeletedAsName
}

func (e CfgFileRemoved) Event() string {
	return CfgFileRemovedAsName
}

func (e CfgFileUpdated) Event() string {
	return CfgFileUpdatedAsName
}

func (e CfgUpdated) Event() string {
	return CfgUpdatedAsName
}

func (e ClientSub) Event() string {
	return ClientSubAsName
}

func (e ClientUnSub) Event() string {
	return ClientUnSubAsName
}

func (e DataUpdated) Bytes() []byte {
	return e.RawMessage
}

func (e DataUpdated) Event() string {
	return DataUpdatedAsName
}

func (e DaemonCtl) Event() string {
	return DaemonCtlAsName
}

func (e Exit) Event() string {
	return ExitAsName
}

func (e Frozen) Event() string {
	return FrozenAsName
}

func (e FrozenFileRemoved) Event() string {
	return FrozenFileRemovedAsName
}

func (e FrozenFileUpdated) Event() string {
	return FrozenFileUpdatedAsName
}

func (e HbNodePing) Bytes() []byte {
	if e.Status {
		return []byte(e.Node + " ok")
	} else {
		return []byte(e.Node + " stale")
	}
}

func (e HbNodePing) Event() string {
	return HbNodePingAsName
}

func (e HbPing) Bytes() []byte {
	s := fmt.Sprintf("node %s ping detected from %s %s", e.Nodename, e.HbId, e.Time)
	return []byte(s)
}

func (e HbPing) Event() string {
	return HbPingAsName
}

func (e HbStale) Bytes() []byte {
	s := fmt.Sprintf("node %s stale detected from %s %s", e.Nodename, e.HbId, e.Time)
	return []byte(s)
}

func (e HbStale) Event() string {
	return HbStaleAsName
}

func (e HbStatusUpdated) Event() string {
	return HbStatusUpdatedAsName
}

func (e InstanceMonitorDeleted) Event() string {
	return InstanceMonitorDeletedAsName
}

func (e InstanceMonitorUpdated) Event() string {
	return InstanceMonitorUpdatedAsName
}

func (e InstanceStatusDeleted) Event() string {
	return InstanceStatusDeletedAsName
}

func (e InstanceStatusUpdated) Event() string {
	return InstanceStatusUpdatedAsName
}

func (e MonCfgDone) Event() string {
	return MonCfgDoneAsName
}

func (e NodeMonitorDeleted) Event() string {
	return NodeMonitorDeletedAsName
}

func (e NodeMonitorUpdated) Event() string {
	return NodeMonitorUpdatedAsName
}

func (e NodeOsPathsUpdated) Event() string {
	return NodeOsPathsUpdatedAsName
}

func (e NodeStatusLabelsUpdated) Event() string {
	return NodeStatusLabelsUpdatedAsName
}

func (e NodeStatusUpdated) Event() string {
	return NodeStatusUpdatedAsName
}

func (e ObjectAggDeleted) Event() string {
	return ObjectAggDeletedAsName
}

func (e ObjectAggDone) Event() string {
	return ObjectAggDoneAsName
}

func (e ObjectAggUpdated) Bytes() []byte {
	d := e.AggregatedStatus
	s := fmt.Sprintf("%s@%s %s %s %s %s %s %v", e.Path, e.Node, d.Avail, d.Overall, d.Frozen, d.Provisioned, d.Placement, d.Scope)
	return []byte(s)
}

func (e ObjectAggUpdated) Event() string {
	return ObjectAggUpdatedAsName
}

func (e RemoteFileConfig) Event() string {
	return RemoteFileConfigAsName
}

func (e SetInstanceMonitor) Event() string {
	return SetInstanceMonitorAsName
}

func (e SetNodeMonitor) Event() string {
	return SetNodeMonitorAsName
}
