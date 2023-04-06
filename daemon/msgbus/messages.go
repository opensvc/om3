package msgbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/nodesinfo"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/san"
)

var (
	kindToT = map[string]func() any{
		"ApiClient": func() any { return &ApiClient{} },

		"ArbitratorError": func() any { return &ArbitratorError{} },

		"ClusterConfigUpdated": func() any { return &ClusterConfigUpdated{} },

		"ClusterStatusUpdated": func() any { return &ClusterStatusUpdated{} },

		"ConfigFileRemoved": func() any { return &ConfigFileRemoved{} },

		"ConfigFileUpdated": func() any { return &ConfigFileUpdated{} },

		"ClientSub": func() any { return &ClientSub{} },

		"ClientUnSub": func() any { return &ClientUnSub{} },

		"DaemonCtl": func() any { return &DaemonCtl{} },

		"DataUpdated": func() any { return &DataUpdated{} },

		"Exit": func() any { return &Exit{} },

		"ForgetPeer": func() any { return &ForgetPeer{} },

		"HbMessageTypeUpdated": func() any { return &HbMessageTypeUpdated{} },

		"HbNodePing": func() any { return &HbNodePing{} },

		"HbPing": func() any { return &HbPing{} },

		"HbStale": func() any { return &HbStale{} },

		"HbStatusUpdated": func() any { return &HbStatusUpdated{} },

		"InstanceConfigDeleted": func() any { return &InstanceConfigDeleted{} },

		"InstanceConfigUpdated": func() any { return &InstanceConfigUpdated{} },

		"InstanceFrozenFileRemoved": func() any { return &InstanceFrozenFileRemoved{} },

		"InstanceFrozenFileUpdated": func() any { return &InstanceFrozenFileUpdated{} },

		"InstanceMonitorAction": func() any { return &InstanceMonitorAction{} },

		"InstanceMonitorDeleted": func() any { return &InstanceMonitorDeleted{} },

		"InstanceMonitorUpdated": func() any { return &InstanceMonitorUpdated{} },

		"InstanceStatusDeleted": func() any { return &InstanceStatusDeleted{} },

		"InstanceStatusPost": func() any { return &InstanceStatusPost{} },

		"InstanceStatusUpdated": func() any { return &InstanceStatusUpdated{} },

		"InstanceConfigManagerDone": func() any { return &InstanceConfigManagerDone{} },

		"JoinError": func() any { return &JoinError{} },

		"JoinIgnored": func() any { return &JoinIgnored{} },

		"JoinRequest": func() any { return &JoinRequest{} },

		"JoinSuccess": func() any { return &JoinSuccess{} },

		"LeaveError": func() any { return &LeaveError{} },

		"LeaveIgnored": func() any { return &LeaveIgnored{} },

		"LeaveRequest": func() any { return &LeaveRequest{} },

		"LeaveSuccess": func() any { return &LeaveSuccess{} },

		"NodeConfigUpdated": func() any { return &NodeConfigUpdated{} },

		"NodeFrozen": func() any { return &NodeFrozen{} },

		"NodeFrozenFileUpdated": func() any { return &NodeFrozenFileUpdated{} },

		"NodeMonitorDeleted": func() any { return &NodeMonitorDeleted{} },

		"NodeMonitorUpdated": func() any { return &NodeMonitorUpdated{} },

		"NodeOsPathsUpdated": func() any { return &NodeOsPathsUpdated{} },

		"NodeStatsUpdated": func() any { return &NodeStatsUpdated{} },

		"NodeStatusArbitratorsUpdated": func() any { return &NodeStatusArbitratorsUpdated{} },

		"NodeStatusGenUpdates": func() any { return &NodeStatusGenUpdates{} },

		"NodeStatusLabelsUpdated": func() any { return &NodeStatusLabelsUpdated{} },

		"NodeSplitAction": func() any { return &NodeSplitAction{} },

		"NodeStatusUpdated": func() any { return &NodeStatusUpdated{} },

		"ObjectOrchestrationEnd": func() any { return &ObjectOrchestrationEnd{} },

		"ObjectStatusDeleted": func() any { return &ObjectStatusDeleted{} },

		"ObjectStatusDone": func() any { return &ObjectStatusDone{} },

		"ObjectStatusUpdated": func() any { return &ObjectStatusUpdated{} },

		"ProgressInstanceMonitor": func() any { return &ProgressInstanceMonitor{} },

		"RemoteFileConfig": func() any { return &RemoteFileConfig{} },

		"SetInstanceMonitor": func() any { return &SetInstanceMonitor{} },

		"SetNodeMonitor": func() any { return &SetNodeMonitor{} },

		"SubscriptionError": func() any { return &pubsub.SubscriptionError{} },

		"WatchDog": func() any { return &WatchDog{} },

		"ZoneRecordDeleted": func() any { return &ZoneRecordDeleted{} },

		"ZoneRecordUpdated": func() any { return &ZoneRecordUpdated{} },
	}
)

func KindToT(kind string) (any, error) {
	if f, ok := kindToT[kind]; ok {
		return f(), nil
	}
	return nil, errors.New("can't find type for kind: " + kind)
}

// EventToMessage converts event.Event message as pubsub.Messager
func EventToMessage(ev event.Event) (pubsub.Messager, error) {
	var c pubsub.Messager
	i, err := KindToT(ev.Kind)
	if err != nil {
		return c, errors.New("can't decode " + ev.Kind)
	}
	c = i.(pubsub.Messager)
	err = json.Unmarshal(ev.Data, c)
	return c, err
}

type (
	ApiClient struct {
		Time time.Time
		Name string
	}

	// ArbitratorError message is published when an arbitrator error is detected
	ArbitratorError struct {
		pubsub.Msg
		Node string
		Name string
		Err  error
	}

	// ConfigFileRemoved is emitted by a fs watcher when a .conf file is removed in etc.
	// The imon goroutine listens to this event and updates the daemondata, which in turns emits a InstanceConfigDeleted{} event.
	ConfigFileRemoved struct {
		pubsub.Msg
		Path     path.T
		Filename string
	}

	// ConfigFileUpdated is emitted by a fs watcher when a .conf file is updated or created in etc.
	// The imon goroutine listens to this event and updates the daemondata, which in turns emits a InstanceConfigUpdated{} event.
	ConfigFileUpdated struct {
		pubsub.Msg
		Path     path.T
		Filename string
	}

	ClientSub struct {
		pubsub.Msg
		ApiClient
	}

	ClientUnSub struct {
		pubsub.Msg
		ApiClient
	}

	ClusterConfigUpdated struct {
		pubsub.Msg
		Node         string
		Value        cluster.Config
		NodesAdded   []string
		NodesRemoved []string
	}

	ClusterStatusUpdated struct {
		pubsub.Msg
		Node  string
		Value cluster.Status
	}

	// DataUpdated is a patch of changed data
	DataUpdated struct {
		pubsub.Msg
		json.RawMessage
	}

	DaemonCtl struct {
		pubsub.Msg
		Component string
		Action    string
	}

	Exit struct {
		Path     path.T
		Filename string
	}

	ForgetPeer struct {
		pubsub.Msg
		Node string
	}

	HbNodePing struct {
		pubsub.Msg
		Node   string
		Status bool
	}

	HbPing struct {
		pubsub.Msg
		Nodename string
		HbId     string
		Time     time.Time
	}

	HbMessageTypeUpdated struct {
		pubsub.Msg
		Node  string
		From  string
		To    string
		Nodes []string
		// JoinedNodes are nodes with hb message type patch
		JoinedNodes []string
	}

	HbStale struct {
		pubsub.Msg
		Nodename string
		HbId     string
		Time     time.Time
	}

	HbStatusUpdated struct {
		pubsub.Msg
		Node  string
		Value cluster.HeartbeatStream
	}

	InstanceConfigDeleted struct {
		pubsub.Msg
		Path path.T
		Node string
	}

	InstanceConfigUpdated struct {
		pubsub.Msg
		Path  path.T
		Node  string
		Value instance.Config
	}

	// InstanceFrozenFileUpdated is emitted by a fs watcher, or imon when an instance frozen file is updated or created.
	InstanceFrozenFileUpdated struct {
		pubsub.Msg
		Path     path.T
		Filename string
		Updated  time.Time
	}

	// InstanceFrozenFileRemoved is emitted by a fs watcher or iman when an instance frozen file is removed.
	InstanceFrozenFileRemoved struct {
		pubsub.Msg
		Path     path.T
		Filename string
		Updated  time.Time
	}

	InstanceMonitorAction struct {
		pubsub.Msg
		Path   path.T
		Node   string
		Action instance.MonitorAction
		RID    string
	}

	InstanceMonitorDeleted struct {
		pubsub.Msg
		Path path.T
		Node string
	}

	InstanceMonitorUpdated struct {
		pubsub.Msg
		Path  path.T
		Node  string
		Value instance.Monitor
	}

	InstanceStatusDeleted struct {
		pubsub.Msg
		Path path.T
		Node string
	}

	InstanceStatusPost struct {
		pubsub.Msg
		Path  path.T
		Node  string
		Value instance.Status
	}

	InstanceStatusUpdated struct {
		pubsub.Msg
		Path  path.T
		Node  string
		Value instance.Status
	}

	InstanceConfigManagerDone struct {
		pubsub.Msg
		Path     path.T
		Filename string
	}

	JoinError struct {
		pubsub.Msg
		// Node is a node that can't be added to cluster config nodes
		Node   string
		Reason string
	}

	JoinIgnored struct {
		pubsub.Msg
		// Node is a node that is already in cluster config nodes
		Node string
	}

	JoinRequest struct {
		pubsub.Msg
		// Node is a node to add to cluster config nodes
		Node string
	}

	JoinSuccess struct {
		pubsub.Msg
		// Node is the successfully added node in cluster config nodes
		Node string
	}

	LeaveError struct {
		pubsub.Msg
		// Node is a node that can't be removed from cluster config nodes
		Node   string
		Reason string
	}

	LeaveIgnored struct {
		pubsub.Msg
		// Node is a node that is not in cluster config nodes
		Node string
	}

	LeaveRequest struct {
		pubsub.Msg
		// Node is a node to remove to cluster config nodes
		Node string
	}

	LeaveSuccess struct {
		pubsub.Msg
		// Node is the successfully removed node from cluster config nodes
		Node string
	}

	NodeConfigUpdated struct {
		pubsub.Msg
		Node  string
		Value node.Config
	}

	// NodeFrozen message describe a node frozen state update
	NodeFrozen struct {
		pubsub.Msg
		Node string
		// Status is true when frozen, else false
		Status bool
		// FrozenAt is the time when node has been frozen or zero when not frozen
		FrozenAt time.Time
	}

	// NodeFrozenFileRemoved is emitted by a fs watcher when a frozen file is removed from var.
	// The nmon goroutine listens to this event and updates the daemondata, which in turns emits a NodeFrozen{} event.
	NodeFrozenFileRemoved struct {
		pubsub.Msg
		Filename string
	}

	// NodeFrozenFileUpdated is emitted by a fs watcher when a frozen file is updated or created in var.
	// The nmon goroutine listens to this event and updates the daemondata, which in turns emits a NodeFrozen{} event.
	NodeFrozenFileUpdated struct {
		pubsub.Msg
		Filename string
		Updated  time.Time
	}

	NodeMonitorDeleted struct {
		pubsub.Msg
		Node string
	}

	NodeMonitorUpdated struct {
		pubsub.Msg
		Node  string
		Value node.Monitor
	}

	NodeOsPathsUpdated struct {
		pubsub.Msg
		Node  string
		Value san.Paths
	}

	NodeSplitAction struct {
		pubsub.Msg
		Node            string
		Action          string
		NodeVotes       int
		ArbitratorVotes int
		Voting          int
		ProVoters       int
	}

	NodeStatsUpdated struct {
		pubsub.Msg
		Node  string
		Value node.Stats
	}

	NodeStatusArbitratorsUpdated struct {
		pubsub.Msg
		Node  string
		Value map[string]node.ArbitratorStatus
	}

	// NodeStatusGenUpdates is emitted when then hb message gens are changed
	NodeStatusGenUpdates struct {
		pubsub.Msg
		Node string
		// Value is Node.Status.Gen
		Value map[string]uint64
	}

	NodeStatusLabelsUpdated struct {
		pubsub.Msg
		Node  string
		Value nodesinfo.Labels
	}

	// NodeStatusUpdated is the message that nmon publish when node status is modified.
	// The Value.Gen may be outdated, daemondata has the most recent version of gen.
	NodeStatusUpdated struct {
		pubsub.Msg
		Node  string
		Value node.Status
	}

	ObjectOrchestrationEnd struct {
		pubsub.Msg
		Id    string
		Node  string
		Path  path.T
		Error error
	}

	ObjectStatusDeleted struct {
		pubsub.Msg
		Path path.T
		Node string
	}

	ObjectStatusDone struct {
		pubsub.Msg
		Path path.T
	}

	ObjectStatusUpdated struct {
		pubsub.Msg
		Path  path.T
		Node  string
		Value object.Status
		SrcEv any
	}

	ProgressInstanceMonitor struct {
		pubsub.Msg
		Path      path.T
		Node      string
		State     instance.MonitorState
		SessionId string
		IsPartial bool
	}

	RemoteFileConfig struct {
		pubsub.Msg
		Path     path.T
		Node     string
		Filename string
		Updated  time.Time
		Ctx      context.Context
		Err      chan error
	}

	SetInstanceMonitor struct {
		pubsub.Msg
		Path  path.T
		Node  string
		Value instance.MonitorUpdate
	}

	SetNodeMonitor struct {
		pubsub.Msg
		Node  string
		Value node.MonitorUpdate
	}

	WatchDog struct {
		pubsub.Msg
		Name string
	}

	ZoneRecordDeleted struct {
		pubsub.Msg
		Path    path.T
		Node    string
		Name    string
		Type    string
		TTL     int
		Content string
	}
	ZoneRecordUpdated struct {
		pubsub.Msg
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

func (e *ApiClient) String() string {
	return fmt.Sprintf("%s %s", e.Name, e.Time)
}

func (e *ArbitratorError) Kind() string {
	return "ArbitratorError"
}

func (e *ClusterConfigUpdated) Kind() string {
	return "ClusterConfigUpdated"
}

func (e *ClusterStatusUpdated) Kind() string {
	return "ClusterStatusUpdated"
}

func (e *ConfigFileRemoved) Kind() string {
	return "ConfigFileRemoved"
}

func (e *ConfigFileUpdated) Kind() string {
	return "ConfigFileUpdated"
}

func (e *ClientSub) Kind() string {
	return "ClientSub"
}

func (e *ClientUnSub) Kind() string {
	return "ClientUnSub"
}

func (e *DataUpdated) Bytes() []byte {
	return e.RawMessage
}

func (e *DataUpdated) Kind() string {
	return "DataUpdated"
}

func (e *DaemonCtl) Kind() string {
	return "DaemonCtl"
}

func (e *Exit) Kind() string {
	return "Exit"
}

func (e *ForgetPeer) Kind() string {
	return "forget_peer"
}

func (e *HbMessageTypeUpdated) Kind() string {
	return "HbMessageTypeUpdated"
}

func (e *HbNodePing) String() string {
	if e.Status {
		return e.Node + " ok"
	} else {
		return e.Node + " stale"
	}
}

func (e *HbNodePing) Kind() string {
	return "HbNodePing"
}

func (e *HbPing) String() string {
	s := fmt.Sprintf("node %s ping detected from %s %s", e.Nodename, e.HbId, e.Time)
	return s
}

func (e *HbPing) Kind() string {
	return "HbPing"
}

func (e *HbStale) String() string {
	s := fmt.Sprintf("node %s stale detected from %s %s", e.Nodename, e.HbId, e.Time)
	return s
}

func (e *HbStale) Kind() string {
	return "HbStale"
}

func (e *HbStatusUpdated) Kind() string {
	return "HbStatusUpdated"
}

func (e *InstanceConfigDeleted) Kind() string {
	return "InstanceConfigDeleted"
}

func (e *InstanceConfigUpdated) Kind() string {
	return "InstanceConfigUpdated"
}

func (e *InstanceFrozenFileRemoved) Kind() string {
	return "InstanceFrozenFileRemoved"
}

func (e *InstanceFrozenFileUpdated) Kind() string {
	return "InstanceFrozenFileUpdated"
}

func (e *InstanceMonitorAction) Kind() string {
	return "InstanceMonitorAction"
}

func (e *InstanceMonitorDeleted) Kind() string {
	return "InstanceMonitorDeleted"
}

func (e *InstanceMonitorUpdated) Kind() string {
	return "InstanceMonitorUpdated"
}

func (e *InstanceStatusDeleted) Kind() string {
	return "InstanceStatusDeleted"
}

func (e *InstanceStatusPost) Kind() string {
	return "InstanceStatusPost"
}

func (e *InstanceStatusUpdated) Kind() string {
	return "InstanceStatusUpdated"
}

func (e *InstanceConfigManagerDone) Kind() string {
	return "InstanceConfigManagerDone"
}

func (e *JoinError) Kind() string {
	return "JoinError"
}

func (e *JoinIgnored) Kind() string {
	return "JoinIgnored"
}

func (e *JoinRequest) Kind() string {
	return "JoinRequest"
}

func (e *JoinSuccess) Kind() string {
	return "JoinSuccess"
}

func (e *LeaveError) Kind() string {
	return "LeaveError"
}

func (e *LeaveIgnored) Kind() string {
	return "LeaveIgnored"
}

func (e *LeaveRequest) Kind() string {
	return "LeaveRequest"
}

func (e *LeaveSuccess) Kind() string {
	return "LeaveSuccess"
}

func (e *NodeConfigUpdated) Kind() string {
	return "NodeConfigUpdated"
}

func (e *NodeFrozen) Kind() string {
	return "NodeFrozen"
}

func (e *NodeFrozenFileRemoved) Kind() string {
	return "NodeFrozenFileRemoved"
}

func (e *NodeFrozenFileUpdated) Kind() string {
	return "NodeFrozenFileUpdated"
}

func (e *NodeMonitorDeleted) Kind() string {
	return "NodeMonitorDeleted"
}

func (e *NodeMonitorUpdated) Kind() string {
	return "NodeMonitorUpdated"
}

func (e *NodeOsPathsUpdated) Kind() string {
	return "NodeOsPathsUpdated"
}

func (e *NodeSplitAction) Kind() string {
	return "NodeSplitAction"
}

func (e *NodeStatsUpdated) Kind() string {
	return "NodeStatsUpdated"
}

func (e *NodeStatusArbitratorsUpdated) Kind() string {
	return "NodeStatusArbitratorsUpdated"
}

func (e *NodeStatusGenUpdates) Kind() string {
	return "NodeStatusGenUpdates"
}

func (e *NodeStatusLabelsUpdated) Kind() string {
	return "NodeStatusLabelsUpdated"
}

func (e *NodeStatusUpdated) Kind() string {
	return "NodeStatusUpdated"
}

func (e *ObjectOrchestrationEnd) Kind() string {
	return "ObjectOrchestrationEnd"
}

func (e *ObjectStatusDeleted) Kind() string {
	return "ObjectStatusDeleted"
}

func (e *ObjectStatusDone) Kind() string {
	return "ObjectStatusDone"
}

func (e *ObjectStatusUpdated) String() string {
	d := e.Value
	s := fmt.Sprintf("%s@%s %s %s %s %s %v", e.Path, e.Node, d.Avail, d.Overall, d.Frozen, d.Provisioned, d.Scope)
	return s
}

func (e *ObjectStatusUpdated) Kind() string {
	return "ObjectStatusUpdated"
}

func (e *ProgressInstanceMonitor) Kind() string {
	return "ProgressInstanceMonitor"
}

func (e *RemoteFileConfig) Kind() string {
	return "RemoteFileConfig"
}

func (e *SetInstanceMonitor) Kind() string {
	return "SetInstanceMonitor"
}

func (e *SetNodeMonitor) Kind() string {
	return "SetNodeMonitor"
}

func (e *WatchDog) String() string {
	return e.Name
}

func (e *WatchDog) Kind() string {
	return "WatchDog"
}

func (e *ZoneRecordDeleted) Kind() string {
	return "ZoneRecordDeleted"
}

func (e *ZoneRecordUpdated) Kind() string {
	return "ZoneRecordUpdated"
}
