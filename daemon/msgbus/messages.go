package msgbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/nodesinfo"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/san"
)

var (
	kindToT = map[string]any{
		"ApiClient":               ApiClient{},
		"CfgDeleted":              CfgDeleted{},
		"CfgFileRemoved":          CfgFileRemoved{},
		"CfgFileUpdated":          CfgFileUpdated{},
		"CfgUpdated":              CfgUpdated{},
		"ClientSub":               ClientSub{},
		"ClientUnSub":             ClientUnSub{},
		"DaemonCtl":               DaemonCtl{},
		"DataUpdated":             DataUpdated{},
		"Exit":                    Exit{},
		"FrozenFileRemoved":       FrozenFileRemoved{},
		"FrozenFileUpdated":       FrozenFileUpdated{},
		"Frozen":                  Frozen{},
		"HbNodePing":              HbNodePing{},
		"HbPing":                  HbPing{},
		"HbStale":                 HbStale{},
		"HbStatusUpdated":         HbStatusUpdated{},
		"InstanceMonitorDeleted":  InstanceMonitorDeleted{},
		"InstanceMonitorUpdated":  InstanceMonitorUpdated{},
		"InstanceStatusDeleted":   InstanceStatusDeleted{},
		"InstanceStatusUpdated":   InstanceStatusUpdated{},
		"MonCfgDone":              MonCfgDone{},
		"NodeMonitorDeleted":      NodeMonitorDeleted{},
		"NodeMonitorUpdated":      NodeMonitorUpdated{},
		"NodeOsPathsUpdated":      NodeOsPathsUpdated{},
		"NodeStatusLabelsUpdated": NodeStatusLabelsUpdated{},
		"NodeStatusUpdated":       NodeStatusUpdated{},
		"ObjectAggDeleted":        ObjectAggDeleted{},
		"ObjectAggDone":           ObjectAggDone{},
		"ObjectAggUpdated":        ObjectAggUpdated{},
		"RemoteFileConfig":        RemoteFileConfig{},
		"SetInstanceMonitor":      SetInstanceMonitor{},
		"SetNodeMonitor":          SetNodeMonitor{},
		"WatchDog":                WatchDog{},
	}
)

func KindToT(kind string) (any, error) {
	if v, ok := kindToT[kind]; ok {
		return v, nil
	}
	return nil, errors.New("can't find type for kind: " + kind)
}

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

	WatchDog struct {
		Name string
	}
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

func (e CfgDeleted) Kind() string {
	return "CfgDeleted"
}

func (e CfgFileRemoved) Kind() string {
	return "CfgFileRemoved"
}

func (e CfgFileUpdated) Kind() string {
	return "CfgFileUpdated"
}

func (e CfgUpdated) Kind() string {
	return "CfgUpdated"
}

func (e ClientSub) Kind() string {
	return "ClientSub"
}

func (e ClientUnSub) Kind() string {
	return "ClientUnSub"
}

func (e DataUpdated) Bytes() []byte {
	return e.RawMessage
}

func (e DataUpdated) Kind() string {
	return "DataUpdated"
}

func (e DaemonCtl) Kind() string {
	return "DaemonCtl"
}

func (e Exit) Kind() string {
	return "Exit"
}

func (e Frozen) Kind() string {
	return "Frozen"
}

func (e FrozenFileRemoved) Kind() string {
	return "FrozenFileRemoved"
}

func (e FrozenFileUpdated) Kind() string {
	return "FrozenFileUpdated"
}

func (e HbNodePing) Bytes() []byte {
	if e.Status {
		return []byte(e.Node + " ok")
	} else {
		return []byte(e.Node + " stale")
	}
}

func (e HbNodePing) Kind() string {
	return "HbNodePing"
}

func (e HbPing) Bytes() []byte {
	s := fmt.Sprintf("node %s ping detected from %s %s", e.Nodename, e.HbId, e.Time)
	return []byte(s)
}

func (e HbPing) Kind() string {
	return "HbPing"
}

func (e HbStale) Bytes() []byte {
	s := fmt.Sprintf("node %s stale detected from %s %s", e.Nodename, e.HbId, e.Time)
	return []byte(s)
}

func (e HbStale) Kind() string {
	return "HbStale"
}

func (e HbStatusUpdated) Kind() string {
	return "HbStatusUpdated"
}

func (e InstanceMonitorDeleted) Kind() string {
	return "InstanceMonitorDeleted"
}

func (e InstanceMonitorUpdated) Kind() string {
	return "InstanceMonitorUpdated"
}

func (e InstanceStatusDeleted) Kind() string {
	return "InstanceStatusDeleted"
}

func (e InstanceStatusUpdated) Kind() string {
	return "InstanceStatusUpdated"
}

func (e InstanceStatusUpdated) Bytes() []byte {
	d := e.Status
	s := fmt.Sprintf("%s@%s %s %s %s %s", e.Path, e.Node, d.Avail, d.Overall, d.Frozen, d.Provisioned)
	return []byte(s)
}

func (e MonCfgDone) Kind() string {
	return "MonCfgDoneAsName"
}

func (e NodeMonitorDeleted) Kind() string {
	return "NodeMonitorDeleted"
}

func (e NodeMonitorUpdated) Kind() string {
	return "NodeMonitorUpdated"
}

func (e NodeOsPathsUpdated) Kind() string {
	return "NodeOsPathsUpdated"
}

func (e NodeStatusLabelsUpdated) Kind() string {
	return "NodeStatusLabelsUpdated"
}

func (e NodeStatusUpdated) Kind() string {
	return "NodeStatusUpdated"
}

func (e ObjectAggDeleted) Kind() string {
	return "ObjectAggDeleted"
}

func (e ObjectAggDone) Kind() string {
	return "ObjectAggDone"
}

func (e ObjectAggUpdated) Bytes() []byte {
	d := e.AggregatedStatus
	s := fmt.Sprintf("%s@%s %s %s %s %s %v", e.Path, e.Node, d.Avail, d.Overall, d.Frozen, d.Provisioned, d.Scope)
	return []byte(s)
}

func (e ObjectAggUpdated) Kind() string {
	return "ObjectAggUpdated"
}

func (e RemoteFileConfig) Kind() string {
	return "RemoteFileConfig"
}

func (e SetInstanceMonitor) Kind() string {
	return "SetInstanceMonitor"
}

func (e SetNodeMonitor) Kind() string {
	return "SetNodeMonitor"
}

func (e WatchDog) Bytes() []byte {
	return []byte(e.Name)
}

func (e WatchDog) Kind() string {
	return "WatchDog"
}
