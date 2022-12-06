package msgbus

import (
	"context"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/nodesinfo"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/san"
)

type (
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

	NodeMonitorDeleted struct {
		Node string
	}

	NodeMonitorUpdated struct {
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

	NodeOsPathsUpdated struct {
		Node  string
		Value san.Paths
	}

	SetNodeMonitor struct {
		Node    string
		Monitor cluster.NodeMonitor
	}

	SetInstanceMonitor struct {
		Path    path.T
		Node    string
		Monitor instance.Monitor
	}

	ObjectAggDeleted struct {
		Path path.T
		Node string
	}

	ObjectAggUpdated struct {
		Path             path.T
		Node             string
		AggregatedStatus object.AggregatedStatus
		SrcEv            any
	}

	ObjectAggDone struct {
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

	DaemonCtl struct {
		Component string
		Action    string
	}
)

const (
	ExitAsEventSource                    = "ExitAsEvent"
	CfgFileUpdatedAsEventSource          = "updated object config file"
	CfgFileRemovedAsEventSource          = "deleted object config file"
	FrozenFileUpdatedAsEventSource       = "updated frozen file"
	FrozenFileRemovedAsEventSource       = "deleted frozen file"
	CfgDeletedAsEventSource              = "deleted object config"
	CfgUpdatedAsEventSource              = "updated object config"
	MonCfgDoneAsEventSource              = "done monitor config"
	RemoteFileConfigAsEventSource        = "updated remote config file"
	FrozenAsEventSource                  = "updated frozen"
	InstanceMonitorDeletedAsEventSource  = "deleted instance monitor"
	InstanceMonitorUpdatedAsEventSource  = "updated instance monitor"
	InstanceStatusDeletedAsEventSource   = "deleted instance status"
	InstanceStatusUpdatedAsEventSource   = "updated instance status"
	NodeMonitorDeletedAsEventSource      = "deleted node monitor"
	NodeMonitorUpdatedAsEventSource      = "updated node monitor"
	NodeStatusUpdatedAsEventSource       = "updated node status"
	NodeStatusLabelsUpdatedAsEventSource = "updated node label"
	NodeOsPathsUpdatedAsEventSource      = "updated node os paths"
	SetNodeMonitorAsEventSource          = "set node monitor"
	SetInstanceMonitorAsEventSource      = "set instance monitor"
	ObjectAggDeletedAsEventSource        = "deleted object aggregated status"
	ObjectAggUpdatedAsEventSource        = "updated object aggregated status"
	ObjectAggDoneAsEventSource           = "done object aggregated status"
	HbStatusUpdatedAsEventSource         = "updated hb status"
	HbNodePingAsEventSource              = "updated node ping"
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

func (e Exit) EventSource() string {
	return ExitAsEventSource
}

func (e CfgFileUpdated) EventSource() string {
	return CfgFileUpdatedAsEventSource
}

func (e CfgFileRemoved) EventSource() string {
	return CfgFileRemovedAsEventSource
}

func (e FrozenFileUpdated) EventSource() string {
	return FrozenFileUpdatedAsEventSource
}

func (e FrozenFileRemoved) EventSource() string {
	return FrozenFileRemovedAsEventSource
}

func (e CfgDeleted) EventSource() string {
	return CfgDeletedAsEventSource
}

func (e CfgUpdated) EventSource() string {
	return CfgUpdatedAsEventSource
}

func (e MonCfgDone) EventSource() string {
	return MonCfgDoneAsEventSource
}

func (e RemoteFileConfig) EventSource() string {
	return RemoteFileConfigAsEventSource
}

func (e Frozen) EventSource() string {
	return FrozenAsEventSource
}

func (e InstanceMonitorDeleted) EventSource() string {
	return InstanceMonitorDeletedAsEventSource
}

func (e InstanceMonitorUpdated) EventSource() string {
	return InstanceMonitorUpdatedAsEventSource
}

func (e InstanceStatusDeleted) EventSource() string {
	return InstanceStatusDeletedAsEventSource
}

func (e InstanceStatusUpdated) EventSource() string {
	return InstanceStatusUpdatedAsEventSource
}

func (e NodeMonitorDeleted) EventSource() string {
	return NodeMonitorDeletedAsEventSource
}

func (e NodeMonitorUpdated) EventSource() string {
	return NodeMonitorUpdatedAsEventSource
}

func (e NodeStatusUpdated) EventSource() string {
	return NodeStatusUpdatedAsEventSource
}

func (e NodeStatusLabelsUpdated) EventSource() string {
	return NodeStatusLabelsUpdatedAsEventSource
}

func (e NodeOsPathsUpdated) EventSource() string {
	return NodeOsPathsUpdatedAsEventSource
}

func (e SetNodeMonitor) EventSource() string {
	return SetNodeMonitorAsEventSource
}

func (e SetInstanceMonitor) EventSource() string {
	return SetInstanceMonitorAsEventSource
}

func (e ObjectAggDeleted) EventSource() string {
	return ObjectAggDeletedAsEventSource
}

func (e ObjectAggUpdated) EventSource() string {
	return ObjectAggUpdatedAsEventSource
}

func (e ObjectAggDone) EventSource() string {
	return ObjectAggDoneAsEventSource
}

func (e HbStatusUpdated) EventSource() string {
	return HbStatusUpdatedAsEventSource
}

func (e HbNodePing) EventSource() string {
	return HbNodePingAsEventSource
}
