package msgbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/nodesinfo"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/san"
)

var (
	kindToT = map[string]any{
		"ApiClient": ApiClient{},

		"ArbitratorError": ArbitratorError{},

		"ClusterConfigUpdated": ClusterConfigUpdated{},

		"ClusterStatusUpdated": ClusterStatusUpdated{},

		"ConfigDeleted": ConfigDeleted{},

		"ConfigFileRemoved": ConfigFileRemoved{},

		"ConfigFileUpdated": ConfigFileUpdated{},

		"ConfigUpdated": ConfigUpdated{},

		"ClientSub": ClientSub{},

		"ClientUnSub": ClientUnSub{},

		"DaemonCtl": DaemonCtl{},

		"DataUpdated": DataUpdated{},

		"Exit": Exit{},

		"ForgetPeer": ForgetPeer{},

		"FrozenFileRemoved": FrozenFileRemoved{},

		"FrozenFileUpdated": FrozenFileUpdated{},

		"Frozen": Frozen{},

		"HbMessageTypeUpdated": HbMessageTypeUpdated{},

		"HbNodePing": HbNodePing{},

		"HbPing": HbPing{},

		"HbStale": HbStale{},

		"HbStatusUpdated": HbStatusUpdated{},

		"InstanceMonitorAction": InstanceMonitorAction{},

		"InstanceMonitorDeleted": InstanceMonitorDeleted{},

		"InstanceMonitorUpdated": InstanceMonitorUpdated{},

		"InstanceStatusDeleted": InstanceStatusDeleted{},

		"InstanceStatusUpdated": InstanceStatusUpdated{},

		"InstanceConfigManagerDone": InstanceConfigManagerDone{},

		"JoinError": JoinError{},

		"JoinIgnored": JoinIgnored{},

		"JoinRequest": JoinRequest{},

		"JoinSuccess": JoinSuccess{},

		"LeaveError": LeaveError{},

		"LeaveIgnored": LeaveIgnored{},

		"LeaveRequest": LeaveRequest{},

		"LeaveSuccess": LeaveSuccess{},

		"NodeConfigUpdated": NodeConfigUpdated{},

		"NodeMonitorDeleted": NodeMonitorDeleted{},

		"NodeMonitorUpdated": NodeMonitorUpdated{},

		"NodeOsPathsUpdated": NodeOsPathsUpdated{},

		"NodeStatsUpdated": NodeStatsUpdated{},

		"NodeStatusLabelsUpdated": NodeStatusLabelsUpdated{},

		"NodeSplitAction": NodeSplitAction{},

		"NodeStatusUpdated": NodeStatusUpdated{},

		"ObjectOrchestrationEnd": ObjectOrchestrationEnd{},

		"ObjectStatusDeleted": ObjectStatusDeleted{},

		"ObjectStatusDone": ObjectStatusDone{},

		"ObjectStatusUpdated": ObjectStatusUpdated{},

		"ProgressInstanceMonitor": ProgressInstanceMonitor{},

		"RemoteFileConfig": RemoteFileConfig{},

		"SetInstanceMonitor": SetInstanceMonitor{},

		"SetNodeMonitor": SetNodeMonitor{},

		"SubscriptionError": pubsub.SubscriptionError{},

		"WatchDog": WatchDog{},

		"ZoneRecordDeleted": ZoneRecordDeleted{},

		"ZoneRecordUpdated": ZoneRecordUpdated{},
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

	// ArbitratorError message is published when an arbitrator error is detected
	ArbitratorError struct {
		Node string
		Name string
		Err  error
	}

	ConfigDeleted struct {
		Path path.T
		Node string
	}

	// ConfigFileRemoved is emitted by a fs watcher when a .conf file is removed in etc.
	// The imon goroutine listens to this event and updates the daemondata, which in turns emits a ConfigDeleted{} event.
	ConfigFileRemoved struct {
		Path     path.T
		Filename string
	}

	// ConfigFileUpdated is emitted by a fs watcher when a .conf file is updated or created in etc.
	// The imon goroutine listens to this event and updates the daemondata, which in turns emits a ConfigUpdated{} event.
	ConfigFileUpdated struct {
		Path     path.T
		Filename string
	}

	ConfigUpdated struct {
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

	ClusterConfigUpdated struct {
		Node         string
		Value        cluster.Config
		NodesAdded   []string
		NodesRemoved []string
	}

	ClusterStatusUpdated struct {
		Node  string
		Value cluster.Status
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
		Value cluster.HeartbeatStream
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

	InstanceConfigManagerDone struct {
		Path     path.T
		Filename string
	}

	JoinError struct {
		// Node is a node that can't be added to cluster config nodes
		Node   string
		Reason string
	}

	JoinIgnored struct {
		// Node is a node that is already in cluster config nodes
		Node string
	}

	JoinRequest struct {
		// Node is a node to add to cluster config nodes
		Node string
	}

	JoinSuccess struct {
		// Node is the successfully added node in cluster config nodes
		Node string
	}

	LeaveError struct {
		// Node is a node that can't be removed from cluster config nodes
		Node   string
		Reason string
	}

	LeaveIgnored struct {
		// Node is a node that is not in cluster config nodes
		Node string
	}

	LeaveRequest struct {
		// Node is a node to remove to cluster config nodes
		Node string
	}

	LeaveSuccess struct {
		// Node is the successfully removed node from cluster config nodes
		Node string
	}

	NodeConfigUpdated struct {
		Node  string
		Value node.Config
	}

	NodeMonitorDeleted struct {
		Node string
	}

	NodeMonitorUpdated struct {
		Node  string
		Value node.Monitor
	}

	NodeOsPathsUpdated struct {
		Node  string
		Value san.Paths
	}

	NodeSplitAction struct {
		Node            string
		Action          string
		NodeVotes       int
		ArbitratorVotes int
		Voting          int
		ProVoters       int
	}

	NodeStatsUpdated struct {
		Node  string
		Value node.Stats
	}

	NodeStatusLabelsUpdated struct {
		Node  string
		Value nodesinfo.Labels
	}

	NodeStatusUpdated struct {
		Node  string
		Value node.Status
	}

	ObjectOrchestrationEnd struct {
		Id    string
		Node  string
		Path  path.T
		Error error
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
		Value node.MonitorUpdate
	}

	WatchDog struct {
		Name string
	}
	ZoneRecordDeleted struct {
		Path    path.T
		Node    string
		Name    string
		Type    string
		TTL     int
		Content string
	}
	ZoneRecordUpdated struct {
		Path    path.T
		Node    string
		Name    string
		Type    string
		TTL     int
		Content string
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

func (e ArbitratorError) Kind() string {
	return "ArbitratorError"
}

func (e ClusterConfigUpdated) Kind() string {
	return "ClusterConfigUpdated"
}

func (e ClusterStatusUpdated) Kind() string {
	return "ClusterStatusUpdated"
}

func (e ConfigDeleted) Kind() string {
	return "ConfigDeleted"
}

func (e ConfigFileRemoved) Kind() string {
	return "ConfigFileRemoved"
}

func (e ConfigFileUpdated) Kind() string {
	return "ConfigFileUpdated"
}

func (e ConfigUpdated) Kind() string {
	return "ConfigUpdated"
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

func (e InstanceConfigManagerDone) Kind() string {
	return "InstanceConfigManagerDone"
}

func (e JoinError) Kind() string {
	return "JoinError"
}

func (e JoinIgnored) Kind() string {
	return "JoinIgnored"
}

func (e JoinRequest) Kind() string {
	return "JoinRequest"
}

func (e JoinSuccess) Kind() string {
	return "JoinSuccess"
}

func (e LeaveError) Kind() string {
	return "LeaveError"
}

func (e LeaveIgnored) Kind() string {
	return "LeaveIgnored"
}

func (e LeaveRequest) Kind() string {
	return "LeaveRequest"
}

func (e LeaveSuccess) Kind() string {
	return "LeaveSuccess"
}

func (e NodeConfigUpdated) Kind() string {
	return "NodeConfigUpdated"
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

func (e NodeSplitAction) Kind() string {
	return "NodeSplitAction"
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

func (e ObjectOrchestrationEnd) Kind() string {
	return "ObjectOrchestrationEnd"
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

func (e ZoneRecordDeleted) Kind() string {
	return "ZoneRecordDeleted"
}

func (e ZoneRecordUpdated) Kind() string {
	return "ZoneRecordUpdated"
}
