package msgbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/san"
)

var (
	kindToT = map[string]func() any{
		"ArbitratorError": func() any { return &ArbitratorError{} },

		"ClusterConfigUpdated": func() any { return &ClusterConfigUpdated{} },

		"ClusterStatusUpdated": func() any { return &ClusterStatusUpdated{} },

		"ConfigFileRemoved": func() any { return &ConfigFileRemoved{} },

		"ConfigFileUpdated": func() any { return &ConfigFileUpdated{} },

		"ClientSubscribed": func() any { return &ClientSubscribed{} },

		"ClientUnsubscribed": func() any { return &ClientUnsubscribed{} },

		"DaemonCtl": func() any { return &DaemonCtl{} },

		"DaemonHb": func() any { return &DaemonHb{} },

		"DaemonStart": func() any { return &DaemonStart{} },

		"Exec": func() any { return &Exec{} },

		"ExecFailed": func() any { return &ExecFailed{} },

		"ExecSuccess": func() any { return &ExecSuccess{} },

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

		"Log": func() any { return &Log{} },

		"NodeConfigUpdated": func() any { return &NodeConfigUpdated{} },

		"NodeDataUpdated": func() any { return &NodeDataUpdated{} },

		"NodeFrozen": func() any { return &NodeFrozen{} },

		"NodeFrozenFileRemoved": func() any { return &NodeFrozenFileRemoved{} },

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

		"ObjectOrchestrationRefused": func() any { return &ObjectOrchestrationRefused{} },

		"ObjectStatusDeleted": func() any { return &ObjectStatusDeleted{} },

		"ObjectStatusDone": func() any { return &ObjectStatusDone{} },

		"ObjectStatusUpdated": func() any { return &ObjectStatusUpdated{} },

		"ProgressInstanceMonitor": func() any { return &ProgressInstanceMonitor{} },

		"NodeRejoin": func() any { return &NodeRejoin{} },

		"RemoteFileConfig": func() any { return &RemoteFileConfig{} },

		"SetInstanceMonitor": func() any { return &SetInstanceMonitor{} },

		"SetInstanceMonitorRefused": func() any { return &SetInstanceMonitorRefused{} },

		"SetNodeMonitor": func() any { return &SetNodeMonitor{} },

		"SubscriptionError": func() any { return &pubsub.SubscriptionError{} },

		"SubscriptionQueueThreshold": func() any { return &pubsub.SubscriptionQueueThreshold{} },

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
	// ArbitratorError message is published when an arbitrator error is detected
	ArbitratorError struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`
		Name       string `json:"name" yaml:"name"`
		ErrS       string `json:"error" yaml:"error"`
	}

	// ConfigFileRemoved is emitted by a fs watcher when a .conf file is removed in etc.
	// The imon goroutine listens to this event and updates the daemondata, which in turns emits a InstanceConfigDeleted{} event.
	ConfigFileRemoved struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T `json:"path" yaml:"path"`
		File       string `json:"file" yaml:"file"`
	}

	// ConfigFileUpdated is emitted by a fs watcher when a .conf file is updated or created in etc.
	// The imon goroutine listens to this event and updates the daemondata, which in turns emits a InstanceConfigUpdated{} event.
	ConfigFileUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T `json:"path" yaml:"path"`
		File       string `json:"file" yaml:"file"`
	}

	ClientSubscribed struct {
		pubsub.Msg `yaml:",inline"`
		Time       time.Time `json:"at" yaml:"at"`
		Name       string    `json:"name" yaml:"name"`
	}

	ClientUnsubscribed struct {
		pubsub.Msg `yaml:",inline"`
		Time       time.Time `json:"at" yaml:"at"`
		Name       string    `json:"name" yaml:"name"`
	}

	ClusterConfigUpdated struct {
		pubsub.Msg   `yaml:",inline"`
		Node         string         `json:"node" yaml:"node"`
		Value        cluster.Config `json:"cluster_config" yaml:"cluster_config"`
		NodesAdded   []string       `json:"nodes_added" yaml:"nodes_added"`
		NodesRemoved []string       `json:"nodes_removed" yaml:"nodes_removed"`
	}

	ClusterStatusUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string         `json:"node" yaml:"node"`
		Value      cluster.Status `json:"cluster_status" yaml:"cluster_status"`
	}

	DaemonCtl struct {
		pubsub.Msg `yaml:",inline"`
		Component  string `json:"component" yaml:"component"`
		Action     string `json:"action" yaml:"action"`
	}

	DaemonHb struct {
		pubsub.Msg `yaml:",inline"`
		Node       string           `json:"node" yaml:"node"`
		Value      cluster.DaemonHb `json:"hb" yaml:"hb"`
	}

	DaemonStart struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`
		Version    string `json:"version" yaml:"version"`
	}

	// Exec message describes an exec call
	Exec struct {
		pubsub.Msg `yaml:",inline"`
		Command    string `json:"command" yaml:"command"`
		// Node is the nodename that will call exec
		Node string `json:"node" yaml:"node"`
		// Origin describes the exec caller: example: imon, nmon, scheduler...
		Origin string `json:"origin" yaml:"origin"`
		Title  string `json:"title" yaml:"title"`
	}

	// ExecFailed message describes failed exec call
	ExecFailed struct {
		pubsub.Msg `yaml:",inline"`
		Command    string        `json:"command" yaml:"command"`
		Duration   time.Duration `json:"duration" yaml:"duration"`
		ErrS       string        `json:"error" yaml:"error"`
		// Node is the nodename that called exec
		Node string `json:"node" yaml:"node"`
		// Origin describes the exec caller: example: imon, nmon, scheduler...
		Origin string `json:"origin" yaml:"origin"`
		Title  string `json:"title" yaml:"title"`
	}

	// ExecSuccess message describes successfully exec call
	ExecSuccess struct {
		pubsub.Msg `yaml:",inline"`
		Command    string        `json:"command" yaml:"command"`
		Duration   time.Duration `json:"duration" yaml:"duration"`
		// Node is the nodename that called exec
		Node string `json:"node" yaml:"node"`
		// Origin describes the exec caller: example: imon, nmon, scheduler...
		Origin string `json:"origin" yaml:"origin"`
		Title  string `json:"title" yaml:"title"`
	}

	Exit struct {
		Path path.T `json:"path" yaml:"path"`
		File string `json:"file" yaml:"file"`
	}

	ForgetPeer struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`
	}

	HbNodePing struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`
		IsAlive    bool   `json:"is_alive" yaml:"is_alive"`
	}

	HbPing struct {
		pubsub.Msg `yaml:",inline"`
		Nodename   string    `json:"to" yaml:"to"`
		HbId       string    `json:"hb_id" yaml:"hb_id"`
		Time       time.Time `json:"at" yaml:"at"`
	}

	HbMessageTypeUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string   `json:"node" yaml:"node"`
		From       string   `json:"old_type" yaml:"old_type"`
		To         string   `json:"new_type" yaml:"new_type"`
		Nodes      []string `json:"nodes" yaml:"nodes"`

		// JoinedNodes are nodes with hb message type patch
		JoinedNodes []string `json:"joined_nodes" yaml:"joined_nodes"`

		// InstalledGens are the current installed node gens
		InstalledGens map[string]uint64 `json:"installed_gens" yaml:"installed_gens"`
	}

	HbStale struct {
		pubsub.Msg `yaml:",inline"`
		Nodename   string    `json:"node" yaml:"node"`
		HbId       string    `json:"hb_id" yaml:"hb_id"`
		Time       time.Time `json:"at" yaml:"at"`
	}

	HbStatusUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string                  `json:"node" yaml:"node"`
		Value      cluster.HeartbeatStream `json:"stream" yaml:"stream"`
	}

	InstanceConfigDeleted struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T `json:"path" yaml:"path"`
		Node       string `json:"node" yaml:"node"`
	}

	InstanceConfigUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T          `json:"path" yaml:"path"`
		Node       string          `json:"node" yaml:"node"`
		Value      instance.Config `json:"instance_config" yaml:"instance_config"`
	}

	// InstanceFrozenFileUpdated is emitted by a fs watcher, or imon when an instance frozen file is updated or created.
	InstanceFrozenFileUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T    `json:"path" yaml:"path"`
		File       string    `json:"file" yaml:"file"`
		At         time.Time `json:"at" yaml:"at"`
	}

	// InstanceFrozenFileRemoved is emitted by a fs watcher or iman when an instance frozen file is removed.
	InstanceFrozenFileRemoved struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T    `json:"path" yaml:"path"`
		File       string    `json:"file" yaml:"file"`
		At         time.Time `json:"at" yaml:"at"`
	}

	InstanceMonitorAction struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T                 `json:"path" yaml:"path"`
		Node       string                 `json:"node" yaml:"node"`
		Action     instance.MonitorAction `json:"action" yaml:"action"`
		RID        string                 `json:"rid" yaml:"rid"`
	}

	InstanceMonitorDeleted struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T `json:"path" yaml:"path"`
		Node       string `json:"node" yaml:"node"`
	}

	InstanceMonitorUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T           `json:"path" yaml:"path"`
		Node       string           `json:"node" yaml:"node"`
		Value      instance.Monitor `json:"instance_monitor" yaml:"instance_monitor"`
	}

	InstanceStatusDeleted struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T `json:"path" yaml:"path"`
		Node       string `json:"node" yaml:"node"`
	}

	InstanceStatusPost struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T          `json:"path" yaml:"path"`
		Node       string          `json:"node" yaml:"node"`
		Value      instance.Status `json:"instance_status" yaml:"instance_status"`
	}

	InstanceStatusUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T          `json:"path" yaml:"path"`
		Node       string          `json:"node" yaml:"node"`
		Value      instance.Status `json:"instance_status" yaml:"instance_status"`
	}

	InstanceConfigManagerDone struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T `json:"path" yaml:"path"`
		File       string `json:"file" yaml:"file"`
	}

	JoinError struct {
		pubsub.Msg `yaml:",inline"`
		// Node is a node that can't be added to cluster config nodes
		Node   string `json:"node" yaml:"node"`
		Reason string `json:"reason" yaml:"reason"`
	}

	JoinIgnored struct {
		pubsub.Msg `yaml:",inline"`
		// Node is a node that is already in cluster config nodes
		Node string `json:"node" yaml:"node"`
	}

	JoinRequest struct {
		pubsub.Msg `yaml:",inline"`
		// Node is a node to add to cluster config nodes
		Node string `json:"node" yaml:"node"`
	}

	JoinSuccess struct {
		pubsub.Msg `yaml:",inline"`
		// Node is the successfully added node in cluster config nodes
		Node string `json:"node" yaml:"node"`
	}

	LeaveError struct {
		pubsub.Msg `yaml:",inline"`
		// Node is a node that can't be removed from cluster config nodes
		Node   string `json:"node" yaml:"node"`
		Reason string
	}

	LeaveIgnored struct {
		pubsub.Msg `yaml:",inline"`
		// Node is a node that is not in cluster config nodes
		Node string `json:"node" yaml:"node"`
	}

	LeaveRequest struct {
		pubsub.Msg `yaml:",inline"`
		// Node is a node to remove to cluster config nodes
		Node string `json:"node" yaml:"node"`
	}

	LeaveSuccess struct {
		pubsub.Msg `yaml:",inline"`
		// Node is the successfully removed node from cluster config nodes
		Node string `json:"node" yaml:"node"`
	}

	// Log is a log message.
	//
	// Usage example:
	// labels := []pubsub.Label{{"subsystem", "imon"}, {"path", p.String()}}
	// pubsubBus.Pub(&msgbus.Log{Message: "orchestrate", Level: "debug"}, labels...)
	Log struct {
		pubsub.Msg `yaml:",inline"`
		Message    string `json:"message" yaml:"message"`
		Level      string `json:"level" yaml:"level"`
	}

	NodeConfigUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string      `json:"node" yaml:"node"`
		Value      node.Config `json:"node_config" yaml:"node_config"`
	}

	NodeDataUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string    `json:"node" yaml:"node"`
		Value      node.Node `json:"node_data" yaml:"node_data"`
	}

	// NodeFrozen message describe a node frozen state update
	NodeFrozen struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`

		// Status is true when frozen, else false
		Status bool `json:"is_frozen" yaml:"is_frozen"`

		// FrozenAt is the time when node has been frozen or zero when not frozen
		FrozenAt time.Time `json:"frozen_at" yaml:"frozen_at"`
	}

	// NodeFrozenFileRemoved is emitted by a fs watcher when a frozen file is removed from var.
	// The nmon goroutine listens to this event and updates the daemondata, which in turns emits a NodeFrozen{} event.
	NodeFrozenFileRemoved struct {
		pubsub.Msg `yaml:",inline"`
		File       string `json:"file" yaml:"file"`
	}

	// NodeFrozenFileUpdated is emitted by a fs watcher when a frozen file is updated or created in var.
	// The nmon goroutine listens to this event and updates the daemondata, which in turns emits a NodeFrozen{} event.
	NodeFrozenFileUpdated struct {
		pubsub.Msg `yaml:",inline"`
		File       string    `json:"file" yaml:"file"`
		At         time.Time `json:"at" yaml:"at"`
	}

	NodeMonitorDeleted struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`
	}

	NodeMonitorUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string       `json:"node" yaml:"node"`
		Value      node.Monitor `json:"node_monitor" yaml:"node_monitor"`
	}

	NodeOsPathsUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string    `json:"node" yaml:"node"`
		Value      san.Paths `json:"san_paths" yaml:"san_paths"`
	}

	NodeSplitAction struct {
		pubsub.Msg      `yaml:",inline"`
		Node            string `json:"node" yaml:"node"`
		Action          string `json:"action" yaml:"action"`
		NodeVotes       int    `json:"node_votes" yaml:"node_votes"`
		ArbitratorVotes int    `json:"arbitrator_votes" yaml:"arbitrator_votes"`
		Voting          int    `json:"voting" yaml:"voting"`
		ProVoters       int    `json:"pro_voters" yaml:"pro_voters"`
	}

	NodeStatsUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string     `json:"node" yaml:"node"`
		Value      node.Stats `json:"node_stats" yaml:"node_stats"`
	}

	NodeStatusArbitratorsUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string                           `json:"node" yaml:"node"`
		Value      map[string]node.ArbitratorStatus `json:"arbitrator_status" yaml:"arbitrator_status"`
	}

	// NodeStatusGenUpdates is emitted when then hb message gens are changed
	NodeStatusGenUpdates struct {
		pubsub.Msg `yaml:",inline"`
		Node       string
		// Value is Node.Status.Gen
		Value map[string]uint64 `json:"gens" yaml:"gens"`
	}

	NodeStatusLabelsUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string      `json:"node" yaml:"node"`
		Value      node.Labels `json:"node_labels" yaml:"node_labels"`
	}

	// NodeStatusUpdated is the message that nmon publish when node status is modified.
	// The Value.Gen may be outdated, daemondata has the most recent version of gen.
	NodeStatusUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string
		Value      node.Status `json:"node_status" yaml:"node_status"`
	}

	ObjectOrchestrationEnd struct {
		pubsub.Msg `yaml:",inline"`
		Id         string `json:"id" yaml:"id"`
		Node       string `json:"node" yaml:"node"`
		Path       path.T `json:"path" yaml:"path"`
	}

	ObjectOrchestrationRefused struct {
		pubsub.Msg `yaml:",inline"`
		Id         string `json:"id" yaml:"id"`
		Node       string `json:"node" yaml:"node"`
		Path       path.T `json:"path" yaml:"path"`
		Reason     string `json:"reason" yaml:"reason"`
	}

	ObjectStatusDeleted struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T `json:"path" yaml:"path"`
		Node       string `json:"node" yaml:"node"`
	}

	ObjectStatusDone struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T `json:"path" yaml:"path"`
	}

	ObjectStatusUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T        `json:"path" yaml:"path"`
		Node       string        `json:"node" yaml:"node"`
		Value      object.Status `json:"object_status" yaml:"object_status"`
		SrcEv      any           `json:"source_event" yaml:"source_event"`
	}

	ProgressInstanceMonitor struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T                `json:"path" yaml:"path"`
		Node       string                `json:"node" yaml:"node"`
		State      instance.MonitorState `json:"instance_monitor_state" yaml:"instance_monitor_state"`
		SessionId  uuid.UUID             `json:"sessionId" yaml:"sessionId"`
		IsPartial  bool                  `json:"is_partial" yaml:"is_partial"`
	}

	NodeRejoin struct {
		pubsub.Msg     `yaml:",inline"`
		IsUpgrading    bool
		LastShutdownAt time.Time
		Nodes          []string
	}

	RemoteFileConfig struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T          `json:"path" yaml:"path"`
		Node       string          `json:"node" yaml:"node"`
		File       string          `json:"file" yaml:"file"`
		UpdatedAt  time.Time       `json:"updated_at" yaml:"updated_at"`
		Ctx        context.Context `json:"-" yaml:"-"`
		Err        chan error      `json:"-" yaml:"-"`
	}

	SetInstanceMonitor struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T                 `json:"path" yaml:"path"`
		Node       string                 `json:"node" yaml:"node"`
		Value      instance.MonitorUpdate `json:"instance_monitor_update" yaml:"instance_monitor_update"`
		Err        chan error             `json:"-" yaml:"-"`
	}

	SetInstanceMonitorRefused struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T                 `json:"path" yaml:"path"`
		Node       string                 `json:"node" yaml:"node"`
		Value      instance.MonitorUpdate `json:"instance_monitor_update" yaml:"instance_monitor_update"`
	}

	SetNodeMonitor struct {
		pubsub.Msg `yaml:",inline"`
		Node       string             `json:"node" yaml:"node"`
		Value      node.MonitorUpdate `json:"node_monitor_update" yaml:"node_monitor_update"`
		Err        chan error         `json:"-" yaml:"-"`
	}

	WatchDog struct {
		pubsub.Msg `yaml:",inline"`
		Bus        string `json:"bus" yaml:"bus"`
	}

	ZoneRecordDeleted struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T `json:"path" yaml:"path"`
		Node       string `json:"node" yaml:"node"`
		Name       string `json:"name" yaml:"name"`
		Type       string `json:"type" yaml:"type"`
		TTL        int    `json:"ttl" yaml:"ttl"`
		Content    string `json:"content" yaml:"content"`
	}
	ZoneRecordUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       path.T `json:"path" yaml:"path"`
		Node       string `json:"node" yaml:"node"`
		Name       string `json:"name" yaml:"name"`
		Type       string `json:"type" yaml:"type"`
		TTL        int    `json:"ttl" yaml:"ttl"`
		Content    string `json:"content" yaml:"content"`
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

func (e *ClientSubscribed) Kind() string {
	return "ClientSubscribed"
}

func (e *ClientSubscribed) String() string {
	return fmt.Sprintf("%s %s", e.Name, e.Time)
}

func (e *ClientUnsubscribed) Kind() string {
	return "ClientUnsubscribed"
}

func (e *ClientUnsubscribed) String() string {
	return fmt.Sprintf("%s %s", e.Name, e.Time)
}

func (e *DaemonCtl) Kind() string {
	return "DaemonCtl"
}

func (e *DaemonHb) Kind() string {
	return "DaemonHb"
}

func (e *DaemonStart) Kind() string {
	return "DaemonStart"
}

func (e *Exec) Kind() string {
	return "Exec"
}

func (e *ExecFailed) Kind() string {
	return "ExecFailed"
}

func (e *ExecSuccess) Kind() string {
	return "ExecSuccess"
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
	if e.IsAlive {
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

func (e *Log) Kind() string {
	return "Log"
}

func (e *NodeConfigUpdated) Kind() string {
	return "NodeConfigUpdated"
}

func (e *NodeDataUpdated) Kind() string {
	return "NodeDataUpdated"
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

func (e *ObjectOrchestrationRefused) Kind() string {
	return "ObjectOrchestrationRefused"
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

func (e *NodeRejoin) Kind() string {
	return "NodeRejoin"
}

func (e *RemoteFileConfig) Kind() string {
	return "RemoteFileConfig"
}

func (e *SetInstanceMonitor) Kind() string {
	return "SetInstanceMonitor"
}

func (e *SetInstanceMonitorRefused) Kind() string {
	return "SetInstanceMonitorRefused"
}

func (e *SetNodeMonitor) Kind() string {
	return "SetNodeMonitor"
}

func (e *WatchDog) String() string {
	return e.Bus
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
