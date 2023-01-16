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
	"opensvc.com/opensvc/util/pubsub"
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
		"JoinRequest":             JoinRequest{},
		"JoinDenied":              JoinDenied{},
		"JoinIgnored":             JoinIgnored{},
		"NodeAdded":               NodeAdded{},
		"DataUpdated":             DataUpdated{},
		"Exit":                    Exit{},
		"ForgetPeer":              ForgetPeer{},
		"FrozenFileRemoved":       FrozenFileRemoved{},
		"FrozenFileUpdated":       FrozenFileUpdated{},
		"Frozen":                  Frozen{},
		"HbMessageTypeUpdated":    HbMessageTypeUpdated{},
		"HbNodePing":              HbNodePing{},
		"HbPing":                  HbPing{},
		"HbStale":                 HbStale{},
		"HbStatusUpdated":         HbStatusUpdated{},
		"InstanceMonitorAction":   InstanceMonitorAction{},
		"InstanceMonitorDeleted":  InstanceMonitorDeleted{},
		"InstanceMonitorUpdated":  InstanceMonitorUpdated{},
		"InstanceStatusDeleted":   InstanceStatusDeleted{},
		"InstanceStatusUpdated":   InstanceStatusUpdated{},
		"MonCfgDone":              MonCfgDone{},
		"NodeMonitorDeleted":      NodeMonitorDeleted{},
		"NodeMonitorUpdated":      NodeMonitorUpdated{},
		"NodeOsPathsUpdated":      NodeOsPathsUpdated{},
		"NodeStatsUpdated":        NodeStatsUpdated{},
		"NodeStatusLabelsUpdated": NodeStatusLabelsUpdated{},
		"NodeStatusUpdated":       NodeStatusUpdated{},
		"ObjectStatusDeleted":     ObjectStatusDeleted{},
		"ObjectStatusDone":        ObjectStatusDone{},
		"ObjectStatusUpdated":     ObjectStatusUpdated{},
		"ProgressInstanceMonitor": ProgressInstanceMonitor{},
		"RemoteFileConfig":        RemoteFileConfig{},
		"SetInstanceMonitor":      SetInstanceMonitor{},
		"SetNodeMonitor":          SetNodeMonitor{},
		"SubscriptionError":       pubsub.SubscriptionError{},
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
	// The imon goroutine listens to this event and updates the daemondata, which in turns emits a CfgDeleted{} event.
	CfgFileRemoved struct {
		Path     path.T
		Filename string
	}

	// CfgFileUpdated is emitted by a fs watcher when a .conf file is updated or created in etc.
	// The imon goroutine listens to this event and updates the daemondata, which in turns emits a CfgUpdated{} event.
	CfgFileUpdated struct {
		Path     path.T
		Filename string
	}

	CfgUpdated struct {
		Path  path.T
		Node  string
		Value instance.Config
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

	JoinDenied struct {
		// Node is nodename whose join request has been denied
		Node   string
		Reason string
	}
	JoinIgnored struct {
		// Node is nodename whose join request has been ignored
		Node   string
		Reason string
	}
	JoinRequest struct {
		// Node is a new node on cluster
		Node string
	}
	NodeAdded struct {
		// Node is the added nodename in cluster config nodes
		Node string
		Path path.T
		// Nodes is the cluster config nodes
		Nodes []string
	}

	Exit struct {
		Path     path.T
		Filename string
	}

	ForgetPeer struct {
		Node string
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

	HbMessageTypeUpdated struct {
		Node  string
		From  string
		To    string
		Nodes []string
		// JoinedNodes are nodes with hb message type patch
		JoinedNodes []string
	}

	HbStale struct {
		Nodename string
		HbId     string
		Time     time.Time
	}

	HbStatusUpdated struct {
		Node  string
		Value cluster.HeartbeatThreadStatus
	}

	InstanceMonitorAction struct {
		Path   path.T
		Node   string
		Action instance.MonitorAction
		RID    string
	}

	InstanceMonitorDeleted struct {
		Path path.T
		Node string
	}

	InstanceMonitorUpdated struct {
		Path  path.T
		Node  string
		Value instance.Monitor
	}

	InstanceStatusDeleted struct {
		Path path.T
		Node string
	}

	InstanceStatusUpdated struct {
		Path  path.T
		Node  string
		Value instance.Status
	}

	MonCfgDone struct {
		Path     path.T
		Filename string
	}

	NodeMonitorDeleted struct {
		Node string
	}

	NodeMonitorUpdated struct {
		Node  string
		Value cluster.NodeMonitor
	}

	NodeOsPathsUpdated struct {
		Node  string
		Value san.Paths
	}

	NodeStatsUpdated struct {
		Node  string
		Value cluster.NodeStats
	}

	NodeStatusLabelsUpdated struct {
		Node  string
		Value nodesinfo.Labels
	}

	NodeStatusUpdated struct {
		Node  string
		Value cluster.NodeStatus
	}

	ObjectStatusDeleted struct {
		Path path.T
		Node string
	}

	ObjectStatusDone struct {
		Path path.T
	}

	ObjectStatusUpdated struct {
		Path  path.T
		Node  string
		Value object.Status
		SrcEv any
	}

	ProgressInstanceMonitor struct {
		Path      path.T
		Node      string
		State     instance.MonitorState
		SessionId string
		IsPartial bool
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
		Path  path.T
		Node  string
		Value instance.MonitorUpdate
	}

	SetNodeMonitor struct {
		Node  string
		Value cluster.NodeMonitorUpdate
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

func (e ApiClient) String() string {
	return fmt.Sprintf("%s %s", e.Name, e.Time)
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

func (e JoinRequest) Kind() string {
	return "JoinRequest"
}
func (e JoinDenied) Kind() string {
	return "JoinDenied"
}
func (e JoinIgnored) Kind() string {
	return "JoinIgnored"
}
func (e NodeAdded) Kind() string {
	return "NodeAdded"
}

func (e Exit) Kind() string {
	return "Exit"
}

func (e ForgetPeer) Kind() string {
	return "forget_peer"
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

func (e HbMessageTypeUpdated) Kind() string {
	return "HbMessageTypeUpdated"
}

func (e HbNodePing) String() string {
	if e.Status {
		return e.Node + " ok"
	} else {
		return e.Node + " stale"
	}
}

func (e HbNodePing) Kind() string {
	return "HbNodePing"
}

func (e HbPing) String() string {
	s := fmt.Sprintf("node %s ping detected from %s %s", e.Nodename, e.HbId, e.Time)
	return s
}

func (e HbPing) Kind() string {
	return "HbPing"
}

func (e HbStale) String() string {
	s := fmt.Sprintf("node %s stale detected from %s %s", e.Nodename, e.HbId, e.Time)
	return s
}

func (e HbStale) Kind() string {
	return "HbStale"
}

func (e HbStatusUpdated) Kind() string {
	return "HbStatusUpdated"
}

func (e InstanceMonitorAction) Kind() string {
	return "InstanceMonitorAction"
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

func (e NodeStatsUpdated) Kind() string {
	return "NodeStatsUpdated"
}

func (e NodeStatusLabelsUpdated) Kind() string {
	return "NodeStatusLabelsUpdated"
}

func (e NodeStatusUpdated) Kind() string {
	return "NodeStatusUpdated"
}

func (e ObjectStatusDeleted) Kind() string {
	return "ObjectStatusDeleted"
}

func (e ObjectStatusDone) Kind() string {
	return "ObjectStatusDone"
}

func (e ObjectStatusUpdated) String() string {
	d := e.Value
	s := fmt.Sprintf("%s@%s %s %s %s %s %v", e.Path, e.Node, d.Avail, d.Overall, d.Frozen, d.Provisioned, d.Scope)
	return s
}

func (e ObjectStatusUpdated) Kind() string {
	return "ObjectStatusUpdated"
}

func (e ProgressInstanceMonitor) Kind() string {
	return "ProgressInstanceMonitor"
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

func (e WatchDog) String() string {
	return e.Name
}

func (e WatchDog) Kind() string {
	return "WatchDog"
}
