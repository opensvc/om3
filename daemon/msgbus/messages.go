// Package msgbus defines the Opensvc messages
//
// HOWTO Add a new msgX:
//
// - [daemon/msgbus/messages.go] Add the msgX type
// - [daemon/msgbus/messages.go] Add the kindToT["msgX"] mapping
// - [daemon/msgbus/messages.go] Add the `Kind() string` implementation
// - [daemon/msgbus/messages.go] Add the `Key() string` implementation to activate the diff event renderer
// - [core/commoncmd/text/node-events/flag/filter] Document msgX
//
// msgX is waitable (om node events --filter msgX --wait)
//
// - [daemon/msgbus/event_cache.go] add a event cache feeder
//
// msgX is exposed in the daemon data:
//
// - [daemon/msgbus/xxx.go] create the ClusterData.onMsgX function
// - [daemon/msgbus/main.go] update the ClusterData.ApplyMessage function
//
// msgX must be sent to peers (to patch):
//
// - [daemon/daemondata/data.go] update the startSubscriptions function
// - [daemon/daemondata/data.go] update the localEventMustBeForwarded function
//
// msgX is received from peer (from patch):
//
// - [daemon/daemondata/apply_patch.go] update the setCacheAndPublish function:
//
// "full" is received from peer, which contains a msgX data
//
// - [daemon/daemondata/apply_full.go] update the applyNodeData function
//
// on peer node delete, we may need to publish msgX delete events
//
// - [daemon/daemondata/node_data.go] update data.dropPeer function
//
// Note:
//
//   - drop peer node must also publish InstanceConfigDeleted, ...
//     => InstanceConfigUpdated needs publish InstanceConfigDeleted
//   - drop peer node may publish empty DaemonXXXUpdated to reset daemon subsystem state
package msgbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/hbsecret"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/util/errcontext"
	"github.com/opensvc/om3/util/label"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/san"
)

type (
	Keyer interface {
		Key() string
	}
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

		"DaemonCollectorUpdated": func() any { return &DaemonCollectorUpdated{} },

		"DaemonCtl": func() any { return &DaemonCtl{} },

		"DaemonDataUpdated": func() any { return &DaemonDataUpdated{} },

		"DaemonDnsUpdated": func() any { return &DaemonDnsUpdated{} },

		"DaemonHeartbeatUpdated": func() any { return &DaemonHeartbeatUpdated{} },

		"DaemonListenerUpdated": func() any { return &DaemonListenerUpdated{} },

		"DaemonRunnerImonUpdated": func() any { return &DaemonRunnerImonUpdated{} },

		"DaemonSchedulerUpdated": func() any { return &DaemonSchedulerUpdated{} },

		"DaemonStatusUpdated": func() any { return &DaemonStatusUpdated{} },

		"EnterOverloadPeriod": func() any { return &EnterOverloadPeriod{} },

		"LeaveOverloadPeriod": func() any { return &LeaveOverloadPeriod{} },

		"Exec": func() any { return &Exec{} },

		"ExecFailed": func() any { return &ExecFailed{} },

		"ExecSuccess": func() any { return &ExecSuccess{} },

		"Exit": func() any { return &Exit{} },

		"ForgetPeer": func() any { return &ForgetPeer{} },

		"HeartbeatMessageTypeUpdated": func() any { return &HeartbeatMessageTypeUpdated{} },

		"HeartbeatAlive": func() any { return &HeartbeatAlive{} },

		"HeartbeatRotateError": func() any { return &HeartbeatRotateError{} },

		"HeartbeatRotateRequest": func() any { return &HeartbeatRotateRequest{} },

		"HeartbeatRotateSuccess": func() any { return &HeartbeatRotateSuccess{} },

		"HeartbeatSecretUpdated": func() any { return &HeartbeatSecretUpdated{} },

		"HeartbeatStale": func() any { return &HeartbeatStale{} },

		"InstanceConfigDeleted": func() any { return &InstanceConfigDeleted{} },

		"InstanceConfigDeleting": func() any { return &InstanceConfigDeleting{} },

		"InstanceConfigFor": func() any { return &InstanceConfigFor{} },

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

		"NodeAlive": func() any { return &NodeAlive{} },

		"NodeConfigUpdated": func() any { return &NodeConfigUpdated{} },

		"NodeDataUpdated": func() any { return &NodeDataUpdated{} },

		"NodeFrozen": func() any { return &NodeFrozen{} },

		"NodeFrozenFileRemoved": func() any { return &NodeFrozenFileRemoved{} },

		"NodeFrozenFileUpdated": func() any { return &NodeFrozenFileUpdated{} },

		"NodeMonitorDeleted": func() any { return &NodeMonitorDeleted{} },

		"NodeMonitorUpdated": func() any { return &NodeMonitorUpdated{} },

		"NodeOsPathsUpdated": func() any { return &NodeOsPathsUpdated{} },

		"NodePoolStatusDeleted": func() any { return &NodePoolStatusDeleted{} },

		"NodePoolStatusUpdated": func() any { return &NodePoolStatusUpdated{} },

		"NodeStale": func() any { return &NodeStale{} },

		"NodeStatsUpdated": func() any { return &NodeStatsUpdated{} },

		"NodeStatusArbitratorsUpdated": func() any { return &NodeStatusArbitratorsUpdated{} },

		"NodeStatusGenUpdates": func() any { return &NodeStatusGenUpdates{} },

		"NodeStatusLabelsCommited": func() any { return &NodeStatusLabelsCommited{} },

		"NodeStatusLabelsUpdated": func() any { return &NodeStatusLabelsUpdated{} },

		"NodeSplitAction": func() any { return &NodeSplitAction{} },

		"NodeStatusUpdated": func() any { return &NodeStatusUpdated{} },

		"ObjectCreated": func() any { return &ObjectCreated{} },

		"ObjectDeleted": func() any { return &ObjectDeleted{} },

		"ObjectOrchestrationAccepted": func() any { return &ObjectOrchestrationAccepted{} },

		"ObjectOrchestrationEnd": func() any { return &ObjectOrchestrationEnd{} },

		"ObjectOrchestrationRefused": func() any { return &ObjectOrchestrationRefused{} },

		"ObjectStatusDeleted": func() any { return &ObjectStatusDeleted{} },

		"ObjectStatusDone": func() any { return &ObjectStatusDone{} },

		"ObjectStatusUpdated": func() any { return &ObjectStatusUpdated{} },

		"ProgressInstanceMonitor": func() any { return &ProgressInstanceMonitor{} },

		"NodeRejoin": func() any { return &NodeRejoin{} },

		"RemoteFileConfig": func() any { return &RemoteFileConfig{} },

		"RunFileRemoved": func() any { return &RunFileRemoved{} },

		"RunFileUpdated": func() any { return &RunFileUpdated{} },

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
	return nil, fmt.Errorf("can't find type for kind: %s", kind)
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
		Path       naming.Path `json:"path" yaml:"path"`
		File       string      `json:"file" yaml:"file"`
	}

	// ConfigFileUpdated is emitted by a fs watcher when a .conf file is updated or created in etc.
	// The imon goroutine listens to this event and updates the daemondata, which in turns emits a InstanceConfigUpdated{} event.
	ConfigFileUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		File       string      `json:"file" yaml:"file"`
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
		Node       string             `json:"node" yaml:"node"`
		Value      clusterdump.Status `json:"cluster_status" yaml:"cluster_status"`
	}

	DaemonCollectorUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`

		Value daemonsubsystem.Collector `json:"collector" yaml:"collector"`
	}

	DaemonCtl struct {
		pubsub.Msg `yaml:",inline"`
		Component  string `json:"component" yaml:"component"`
		Action     string `json:"action" yaml:"action"`
	}

	DaemonDataUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`

		Value daemonsubsystem.Daemondata `json:"daemondata" yaml:"daemondata"`
	}

	DaemonDnsUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`

		Value daemonsubsystem.Dns `json:"dns" yaml:"dns"`
	}

	DaemonHeartbeatUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`

		Value daemonsubsystem.Heartbeat `json:"heartbeat" yaml:"heartbeat"`
	}

	DaemonListenerUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string                   `json:"node" yaml:"node"`
		Value      daemonsubsystem.Listener `json:"listener" yaml:"listener"`
	}

	DaemonRunnerImonUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`

		Value daemonsubsystem.RunnerImon `json:"runner_imon" yaml:"runner_imon"`
	}

	DaemonSchedulerUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`

		Value daemonsubsystem.Scheduler `json:"scheduler" yaml:"scheduler"`
	}

	// DaemonStatusUpdated message informs about main daemon status
	DaemonStatusUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`
		Version    string `json:"version" yaml:"version"`
		Status     string `json:"status" yaml:"status"`
	}

	EnterOverloadPeriod struct {
		pubsub.Msg `yaml:",inline"`
	}

	// Exec message describes an exec call
	Exec struct {
		pubsub.Msg `yaml:",inline"`
		Command    string `json:"command" yaml:"command"`
		// Node is the nodename that will call exec
		Node string `json:"node" yaml:"node"`
		// Origin describes the exec caller: example: imon, nmon, scheduler...
		Origin             string    `json:"origin" yaml:"origin"`
		Title              string    `json:"title" yaml:"title"`
		SessionID          uuid.UUID `json:"session_id" yaml:"session_id"`
		RequesterSessionID uuid.UUID `json:"requester_session_id" yaml:"requester_session_id"`
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
		Origin             string    `json:"origin" yaml:"origin"`
		Title              string    `json:"title" yaml:"title"`
		SessionID          uuid.UUID `json:"session_id" yaml:"session_id"`
		RequesterSessionID uuid.UUID `json:"requester_session_id" yaml:"requester_session_id"`
	}

	// ExecSuccess message describes successfully exec call
	ExecSuccess struct {
		pubsub.Msg `yaml:",inline"`
		Command    string        `json:"command" yaml:"command"`
		Duration   time.Duration `json:"duration" yaml:"duration"`
		// Node is the nodename that called exec
		Node string `json:"node" yaml:"node"`
		// Origin describes the exec caller: example: imon, nmon, scheduler...
		Origin             string    `json:"origin" yaml:"origin"`
		Title              string    `json:"title" yaml:"title"`
		SessionID          uuid.UUID `json:"session_id" yaml:"session_id"`
		RequesterSessionID uuid.UUID `json:"requester_session_id" yaml:"requester_session_id"`
	}

	Exit struct {
		Path naming.Path `json:"path" yaml:"path"`
		File string      `json:"file" yaml:"file"`
	}

	ForgetPeer struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`
	}

	HeartbeatAlive struct {
		pubsub.Msg `yaml:",inline"`
		Nodename   string    `json:"to" yaml:"to"`
		HbID       string    `json:"hb_id" yaml:"hb_id"`
		Time       time.Time `json:"at" yaml:"at"`
	}

	HeartbeatMessageTypeUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string   `json:"node" yaml:"node"`
		From       string   `json:"old_type" yaml:"old_type"`
		To         string   `json:"new_type" yaml:"new_type"`
		Nodes      []string `json:"nodes" yaml:"nodes"`

		// JoinedNodes are nodes with hb message type patch
		JoinedNodes []string `json:"joined_nodes" yaml:"joined_nodes"`

		// InstalledGens are the current installed node gens
		InstalledGens node.Gen `json:"installed_gens" yaml:"installed_gens"`
	}

	HeartbeatRotateError struct {
		pubsub.Msg `yaml:",inline"`
		ID         uuid.UUID `json:"id" yaml:"id"`
		Reason     string    `json:"reason" yaml:"reason"`
	}

	HeartbeatRotateRequest struct {
		pubsub.Msg `yaml:",inline"`
		ID         uuid.UUID `json:"id" yaml:"id"`
	}

	HeartbeatRotateSuccess struct {
		pubsub.Msg `yaml:",inline"`
		ID         uuid.UUID `json:"id" yaml:"id"`
	}

	HeartbeatSecretUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Nodename   string          `json:"nodename" yaml:"nodename"`
		Value      hbsecret.Secret `json:"hb_secret" yaml:"hb_secret"`
	}

	HeartbeatStale struct {
		pubsub.Msg `yaml:",inline"`
		Nodename   string    `json:"node" yaml:"node"`
		HbID       string    `json:"hb_id" yaml:"hb_id"`
		Time       time.Time `json:"at" yaml:"at"`
	}

	InstanceConfigDeleted struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		Node       string      `json:"node" yaml:"node"`
	}

	// InstanceConfigDeleting event is pushed during imon orchestration deleting
	// step.
	InstanceConfigDeleting struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		Node       string      `json:"node" yaml:"node"`
	}

	// InstanceConfigFor message is published by a node during analyse of
	// instance config file that is scoped for foreign nodes (peers).
	InstanceConfigFor struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		Node       string      `json:"node" yaml:"node"`
		// Orchestrate is the config orchestrate value. it may be used by peers
		// just after installation of fetched instance config file
		Orchestrate string `json:"orchestrate" yaml:"orchestrate"`
		// Scope is the list of nodes that have to fetch this config
		Scope []string `json:"scope" yaml:"scope"`
		// UpdatedAt is the config file time stamp
		UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
	}

	InstanceConfigUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path     `json:"path" yaml:"path"`
		Node       string          `json:"node" yaml:"node"`
		Value      instance.Config `json:"instance_config" yaml:"instance_config"`
	}

	// InstanceFrozenFileUpdated is emitted by a fs watcher, or imon when an instance frozen file is updated or created.
	InstanceFrozenFileUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		File       string      `json:"file" yaml:"file"`
		At         time.Time   `json:"at" yaml:"at"`
	}

	// InstanceFrozenFileRemoved is emitted by a fs watcher or iman when an instance frozen file is removed.
	InstanceFrozenFileRemoved struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		File       string      `json:"file" yaml:"file"`
		At         time.Time   `json:"at" yaml:"at"`
	}

	InstanceMonitorAction struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path            `json:"path" yaml:"path"`
		Node       string                 `json:"node" yaml:"node"`
		Action     instance.MonitorAction `json:"action" yaml:"action"`
		RID        string                 `json:"rid" yaml:"rid"`
	}

	InstanceMonitorDeleted struct {
		pubsub.Msg       `yaml:",inline"`
		Path             naming.Path             `json:"path" yaml:"path"`
		Node             string                  `json:"node" yaml:"node"`
		OrchestrationEnd *ObjectOrchestrationEnd `json:"orchestration_end" yaml:"orchestration_end"`
	}

	InstanceMonitorUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path      `json:"path" yaml:"path"`
		Node       string           `json:"node" yaml:"node"`
		Value      instance.Monitor `json:"instance_monitor" yaml:"instance_monitor"`
	}

	InstanceStatusDeleted struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		Node       string      `json:"node" yaml:"node"`
		PeerDropAt time.Time   `json:"peer_drop_at"`
	}

	InstanceStatusPost struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path     `json:"path" yaml:"path"`
		Node       string          `json:"node" yaml:"node"`
		Value      instance.Status `json:"instance_status" yaml:"instance_status"`
	}

	InstanceStatusUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path     `json:"path" yaml:"path"`
		Node       string          `json:"node" yaml:"node"`
		Value      instance.Status `json:"instance_status" yaml:"instance_status"`
	}

	InstanceConfigManagerDone struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		File       string      `json:"file" yaml:"file"`
	}

	JoinError struct {
		pubsub.Msg `yaml:",inline"`
		// CandidateNode is the candidate node that can't be added to cluster config nodes
		CandidateNode string `json:"candidate_node" yaml:"candidate_node"`
		Reason        string `json:"reason" yaml:"reason"`
	}

	JoinIgnored struct {
		pubsub.Msg `yaml:",inline"`
		// CandidateNode is a candidate node that is already present in cluster config nodes
		CandidateNode string `json:"candidate_node" yaml:"candidate_node"`
	}

	JoinRequest struct {
		pubsub.Msg `yaml:",inline"`
		// CandidateNode is a candidate node to add to cluster config nodes
		CandidateNode string `json:"candidate_node" yaml:"candidate_node"`
	}

	JoinSuccess struct {
		pubsub.Msg `yaml:",inline"`
		// AddedNode is a node that has been successfully added to the cluster config nodes
		// after join request or sysadmin cluster config nodes update.
		AddedNode string `json:"added_node" yaml:"added_node"`
	}

	LeaveError struct {
		pubsub.Msg `yaml:",inline"`
		// CandidateNode is a candidate node that can't be removed from cluster config nodes
		CandidateNode string `json:"candidate_node" yaml:"candidate_node"`
		Reason        string
	}

	LeaveIgnored struct {
		pubsub.Msg `yaml:",inline"`
		// CandidateNode is a candidate node that is not a cluster config node
		CandidateNode string `json:"candidate_node" yaml:"candidate_node"`
	}

	LeaveOverloadPeriod struct {
		pubsub.Msg `yaml:",inline"`
	}

	LeaveRequest struct {
		pubsub.Msg `yaml:",inline"`
		// CandidateNode is a node to remove to cluster config nodes
		CandidateNode string `json:"candidate_node" yaml:"candidate_node"`
	}

	LeaveSuccess struct {
		pubsub.Msg `yaml:",inline"`
		// RemovedNode is the successfully removed node from cluster config nodes
		RemovedNode string `json:"removed_node" yaml:"removed_node"`
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

	NodeAlive struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`
	}

	NodeOsPathsUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string    `json:"node" yaml:"node"`
		Value      san.Paths `json:"san_paths" yaml:"san_paths"`
	}

	NodePoolStatusDeleted struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`
		Name       string `json:"name" yaml:"name"`
	}

	NodePoolStatusUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string      `json:"node" yaml:"node"`
		Name       string      `json:"name" yaml:"name"`
		Value      pool.Status `json:"pool_status" yaml:"pool_status"`
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

	NodeStale struct {
		pubsub.Msg `yaml:",inline"`
		Node       string `json:"node" yaml:"node"`
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
		Value node.Gen `json:"gens" yaml:"gens"`
	}

	NodeStatusLabelsCommited struct {
		pubsub.Msg `yaml:",inline"`
		Node       string  `json:"node" yaml:"node"`
		Value      label.M `json:"node_labels" yaml:"node_labels"`
	}

	NodeStatusLabelsUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string  `json:"node" yaml:"node"`
		Value      label.M `json:"node_labels" yaml:"node_labels"`
	}

	// NodeStatusUpdated is the message that nmon publish when node status is modified.
	// The Value.Gen may be outdated, daemondata has the most recent version of gen.
	NodeStatusUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Node       string      `json:"node" yaml:"node"`
		Value      node.Status `json:"node_status" yaml:"node_status"`
	}

	// ObjectCreated is the message published when a new object is detected by
	// localhost.
	ObjectCreated struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		Node       string      `json:"node" yaml:"node"`
	}

	// ObjectDeleted is the message published when an object deletion is
	// detected by localhost.
	ObjectDeleted struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		Node       string      `json:"node" yaml:"node"`
	}

	ObjectOrchestrationAccepted struct {
		pubsub.Msg            `yaml:",inline"`
		ID                    string                       `json:"id" yaml:"id"`
		Node                  string                       `json:"node" yaml:"node"`
		Path                  naming.Path                  `json:"path" yaml:"path"`
		GlobalExpect          instance.MonitorGlobalExpect `json:"global_expect" yaml:"global_expect"`
		GlobalExpectUpdatedAt time.Time                    `json:"global_expect_updated_at" yaml:"global_expect_updated_at"`
		GlobalExpectOptions   any                          `json:"global_expect_options" yaml:"global_expect_options"`
	}

	ObjectOrchestrationEnd struct {
		pubsub.Msg            `yaml:",inline"`
		ID                    string                       `json:"id" yaml:"id"`
		Node                  string                       `json:"node" yaml:"node"`
		Path                  naming.Path                  `json:"path" yaml:"path"`
		GlobalExpect          instance.MonitorGlobalExpect `json:"global_expect" yaml:"global_expect"`
		GlobalExpectUpdatedAt time.Time                    `json:"global_expect_updated_at" yaml:"global_expect_updated_at"`
		GlobalExpectOptions   any                          `json:"global_expect_options" yaml:"global_expect_options"`
		Aborted               bool                         `json:"aborted" yaml:"aborted"`
	}

	ObjectOrchestrationRefused struct {
		pubsub.Msg          `yaml:",inline"`
		ID                  string                        `json:"id" yaml:"id"`
		Node                string                        `json:"node" yaml:"node"`
		Path                naming.Path                   `json:"path" yaml:"path"`
		Reason              string                        `json:"reason" yaml:"reason"`
		GlobalExpect        *instance.MonitorGlobalExpect `json:"global_expect" yaml:"global_expect"`
		GlobalExpectOptions any                           `json:"global_expect_options" yaml:"global_expect_options"`
	}

	ObjectStatusDeleted struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		Node       string      `json:"node" yaml:"node"`
	}

	ObjectStatusDone struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
	}

	ObjectStatusUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path   `json:"path" yaml:"path"`
		Node       string        `json:"node" yaml:"node"`
		Value      object.Status `json:"object_status" yaml:"object_status"`
		SrcEv      any           `json:"source_event" yaml:"source_event"`
	}

	ProgressInstanceMonitor struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path           `json:"path" yaml:"path"`
		Node       string                `json:"node" yaml:"node"`
		State      instance.MonitorState `json:"instance_monitor_state" yaml:"instance_monitor_state"`
		SessionID  uuid.UUID             `json:"session_id" yaml:"session_id"`
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
		Path       naming.Path     `json:"path" yaml:"path"`
		Node       string          `json:"node" yaml:"node"`
		File       string          `json:"file" yaml:"file"`
		Freeze     bool            `json:"freeze" yaml:"freeze"`
		UpdatedAt  time.Time       `json:"updated_at" yaml:"updated_at"`
		Ctx        context.Context `json:"-" yaml:"-"`
		Err        chan error      `json:"-" yaml:"-"`
	}

	// RunFileUpdated is emitted by the fs_watcher when it detects a
	// new or initial resource run file in <var>.
	RunFileUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		File       string      `json:"file" yaml:"file"`
		RID        string      `json:"rid" yaml:"rid"`
		At         time.Time   `json:"at" yaml:"at"`
	}

	// RunFileRemoved is emitted by the fs_watcher when it detects a
	// resource run file is deleted in <var>.
	RunFileRemoved struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		File       string      `json:"file" yaml:"file"`
		RID        string      `json:"rid" yaml:"rid"`
		At         time.Time   `json:"at" yaml:"at"`
	}

	SetInstanceMonitor struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path               `json:"path" yaml:"path"`
		Node       string                    `json:"node" yaml:"node"`
		Value      instance.MonitorUpdate    `json:"instance_monitor_update" yaml:"instance_monitor_update"`
		Err        errcontext.ErrCloseSender `json:"-" yaml:"-"`
	}

	SetInstanceMonitorRefused struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path            `json:"path" yaml:"path"`
		Node       string                 `json:"node" yaml:"node"`
		Value      instance.MonitorUpdate `json:"instance_monitor_update" yaml:"instance_monitor_update"`
	}

	SetNodeMonitor struct {
		pubsub.Msg `yaml:",inline"`
		Node       string                    `json:"node" yaml:"node"`
		Value      node.MonitorUpdate        `json:"node_monitor_update" yaml:"node_monitor_update"`
		Err        errcontext.ErrCloseSender `json:"-" yaml:"-"`
	}

	WatchDog struct {
		pubsub.Msg `yaml:",inline"`
		Bus        string `json:"bus" yaml:"bus"`
	}

	ZoneRecordDeleted struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		Node       string      `json:"node" yaml:"node"`
		Name       string      `json:"name" yaml:"name"`
		Type       string      `json:"type" yaml:"type"`
		TTL        int         `json:"ttl" yaml:"ttl"`
		Content    string      `json:"content" yaml:"content"`
	}
	ZoneRecordUpdated struct {
		pubsub.Msg `yaml:",inline"`
		Path       naming.Path `json:"path" yaml:"path"`
		Node       string      `json:"node" yaml:"node"`
		Name       string      `json:"name" yaml:"name"`
		Type       string      `json:"type" yaml:"type"`
		TTL        int         `json:"ttl" yaml:"ttl"`
		Content    string      `json:"content" yaml:"content"`
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

func (e *ClusterConfigUpdated) Key() string {
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

func (e *ClientUnsubscribed) Kind() string {
	return "ClientUnsubscribed"
}

func (e *DaemonCollectorUpdated) Kind() string {
	return "DaemonCollectorUpdated"
}

func (e *DaemonCollectorUpdated) Key() string {
	return fmt.Sprintf("DaemonCollectorUpdated,node=%s", e.Node)
}

func (e *DaemonCtl) Kind() string {
	return "DaemonCtl"
}

func (e *DaemonDataUpdated) Kind() string {
	return "DaemonDataUpdated"
}

func (e *DaemonDnsUpdated) Kind() string {
	return "DaemonDnsUpdated"
}

func (e *DaemonDnsUpdated) Key() string {
	return fmt.Sprintf("DaemonDnsUpdated,node=%s", e.Node)
}

func (e *DaemonHeartbeatUpdated) Kind() string {
	return "DaemonHeartbeatUpdated"
}

func (e *DaemonHeartbeatUpdated) Key() string {
	return fmt.Sprintf("DaemonHeartbeatUpdated,node=%s", e.Node)
}

func (e *DaemonListenerUpdated) Kind() string {
	return "DaemonListenerUpdated"
}

func (e *DaemonListenerUpdated) Key() string {
	return fmt.Sprintf("DaemonListenerUpdated,node=%s", e.Node)
}

func (e *DaemonRunnerImonUpdated) Kind() string {
	return "DaemonRunnerImonUpdated"
}

func (e *DaemonRunnerImonUpdated) Key() string {
	return fmt.Sprintf("DaemonRunnerImonUpdated,node=%s", e.Node)
}

func (e *DaemonSchedulerUpdated) Kind() string {
	return "DaemonSchedulerUpdated"
}

func (e *DaemonSchedulerUpdated) Key() string {
	return fmt.Sprintf("DaemonSchedulerUpdated,node=%s", e.Node)
}

func (e *DaemonStatusUpdated) Kind() string {
	return "DaemonStatusUpdated"
}

func (e *DaemonStatusUpdated) Key() string {
	return fmt.Sprintf("DaemonStatusUpdated,node=%s", e.Node)
}

func (e *EnterOverloadPeriod) Kind() string {
	return "EnterOverloadPeriod"
}

func (e *LeaveOverloadPeriod) Kind() string {
	return "LeaveOverloadPeriod"
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
	return "ForgetPeer"
}

func (e *HeartbeatMessageTypeUpdated) Kind() string {
	return "HeartbeatMessageTypeUpdated"
}

func (e *HeartbeatMessageTypeUpdated) Key() string {
	return fmt.Sprintf("HeartbeatMessageTypeUpdated,node=%s", e.Node)
}

func (e *HeartbeatAlive) Kind() string {
	return "HeartbeatAlive"
}

func (e *HeartbeatRotateError) Kind() string {
	return "HeartbeatRotateError"
}

func (e *HeartbeatRotateRequest) Kind() string {
	return "HeartbeatRotateRequest"
}

func (e *HeartbeatRotateSuccess) Kind() string {
	return "HeartbeatRotateSuccess"
}

func (e *HeartbeatSecretUpdated) Kind() string {
	return "HeartbeatSecretUpdated"
}

func (e *HeartbeatStale) Kind() string {
	return "HeartbeatStale"
}

func (e *InstanceConfigDeleted) Kind() string {
	return "InstanceConfigDeleted"
}

func (e *InstanceConfigDeleted) KeysToDelete() []string {
	return []string{
		fmt.Sprintf("InstanceConfigUpdated,path=%s,node=%s", e.Path, e.Node),
	}
}

func (e *InstanceConfigDeleting) Kind() string {
	return "InstanceConfigDeleting"
}

func (e *InstanceConfigFor) Kind() string {
	return "InstanceConfigFor"
}

func (e *InstanceConfigUpdated) Kind() string {
	return "InstanceConfigUpdated"
}

func (e *InstanceConfigUpdated) Key() string {
	return fmt.Sprintf("InstanceConfigUpdated,path=%s,node=%s", e.Path, e.Node)
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

func (e *InstanceMonitorDeleted) KeysToDelete() []string {
	return []string{
		fmt.Sprintf("InstanceMonitorUpdated,path=%s,node=%s", e.Path, e.Node),
	}
}

func (e *InstanceMonitorUpdated) Kind() string {
	return "InstanceMonitorUpdated"
}

func (e *InstanceMonitorUpdated) Key() string {
	return fmt.Sprintf("InstanceMonitorUpdated,path=%s,node=%s", e.Path, e.Node)
}

func (e *InstanceStatusDeleted) Kind() string {
	return "InstanceStatusDeleted"
}

func (e *InstanceStatusDeleted) KeysToDelete() []string {
	return []string{
		fmt.Sprintf("InstanceStatusUpdated,path=%s,node=%s", e.Path, e.Node),
	}
}

func (e *InstanceStatusPost) Kind() string {
	return "InstanceStatusPost"
}

func (e *InstanceStatusUpdated) Kind() string {
	return "InstanceStatusUpdated"
}

func (e *InstanceStatusUpdated) Key() string {
	return fmt.Sprintf("InstanceStatusUpdated,path=%s,node=%s", e.Path, e.Node)
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

func (e *NodeAlive) Kind() string {
	return "NodeAlive"
}

func (e *NodeConfigUpdated) Kind() string {
	return "NodeConfigUpdated"
}

func (e *NodeConfigUpdated) Key() string {
	return fmt.Sprintf("NodeConfigUpdated,node=%s", e.Node)
}

func (e *NodeDataUpdated) Kind() string {
	return "NodeDataUpdated"
}

func (e *NodeDataUpdated) Key() string {
	return fmt.Sprintf("NodeDataUpdated,node=%s", e.Node)
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

func (e *NodeMonitorDeleted) KeysToDelete() []string {
	return []string{
		fmt.Sprintf("NodeMonitorUpdated,node=%s", e.Node),
	}
}

func (e *NodeMonitorUpdated) Kind() string {
	return "NodeMonitorUpdated"
}

func (e *NodeMonitorUpdated) Key() string {
	return fmt.Sprintf("NodeMonitorUpdated,node=%s", e.Node)
}

func (e *NodeOsPathsUpdated) Kind() string {
	return "NodeOsPathsUpdated"
}

func (e *NodeOsPathsUpdated) Key() string {
	return fmt.Sprintf("NodeOsPathsUpdated,node=%s", e.Node)
}

func (e *NodeSplitAction) Kind() string {
	return "NodeSplitAction"
}

func (e *NodeStatsUpdated) Kind() string {
	return "NodeStatsUpdated"
}

func (e *NodeStatsUpdated) Key() string {
	return fmt.Sprintf("NodeStatsUpdated,node=%s", e.Node)
}

func (e *NodeStatusArbitratorsUpdated) Kind() string {
	return "NodeStatusArbitratorsUpdated"
}

func (e *NodeStatusArbitratorsUpdated) Key() string {
	return fmt.Sprintf("NodeStatusArbitratorsUpdated,node=%s", e.Node)
}

func (e *NodeStatusGenUpdates) Kind() string {
	return "NodeStatusGenUpdates"
}

func (e *NodeStatusGenUpdates) Key() string {
	return fmt.Sprintf("NodeStatusGenUpdates,node=%s", e.Node)
}

func (e *NodeStatusLabelsCommited) Kind() string {
	return "NodeStatusLabelsCommited"
}

func (e *NodeStatusLabelsUpdated) Kind() string {
	return "NodeStatusLabelsUpdated"
}

func (e *NodeStatusLabelsUpdated) Key() string {
	return fmt.Sprintf("NodeStatusLabelsUpdated,node=%s", e.Node)
}

func (e *NodeStale) Kind() string {
	return "NodeStale"
}

func (e *NodeStatusUpdated) Kind() string {
	return "NodeStatusUpdated"
}

func (e *NodeStatusUpdated) Key() string {
	return fmt.Sprintf("NodeStatusUpdated,node=%s", e.Node)
}

func (e *ObjectCreated) Kind() string {
	return "ObjectCreated"
}

func (e *ObjectDeleted) Kind() string {
	return "ObjectDeleted"
}

func (e *ObjectOrchestrationAccepted) Kind() string {
	return "ObjectOrchestrationAccepted"
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

func (e *ObjectStatusDeleted) KeysToDelete() []string {
	return []string{
		fmt.Sprintf("ObjectStatusUpdated,path=%s", e.Path),
	}
}

func (e *ObjectStatusDone) Kind() string {
	return "ObjectStatusDone"
}

func (e *ObjectStatusUpdated) Key() string {
	return fmt.Sprintf("ObjectStatusUpdated,path=%s", e.Path)
}

func (e *ObjectStatusUpdated) Kind() string {
	return "ObjectStatusUpdated"
}

func (e *NodePoolStatusDeleted) Kind() string {
	return "NodePoolStatusDeleted"
}

func (e *NodePoolStatusUpdated) Key() string {
	return fmt.Sprintf("PoolStatusUpdated,node=%s,name=%s", e.Node, e.Name)
}

func (e *NodePoolStatusUpdated) Kind() string {
	return "NodePoolStatusUpdated"
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

func (e *RunFileRemoved) Kind() string {
	return "RunFileRemoved"
}

func (e *RunFileUpdated) Kind() string {
	return "RunFileUpdated"
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

func (e *WatchDog) Kind() string {
	return "WatchDog"
}

func (e *ZoneRecordDeleted) Kind() string {
	return "ZoneRecordDeleted"
}

func (e *ZoneRecordDeleted) KeysToDelete() []string {
	return []string{
		fmt.Sprintf("ZoneRecordUpdated,path=%s,node=%s,name=%s,type=%s", e.Path, e.Node, e.Name, e.Type),
	}
}

func (e *ZoneRecordUpdated) Kind() string {
	return "ZoneRecordUpdated"
}

func (e *ZoneRecordUpdated) Key() string {
	return fmt.Sprintf("ZoneRecordUpdated,path=%s,node=%s,name=%s,type=%s", e.Path, e.Node, e.Name, e.Type)
}

func NewSetInstanceMonitorWithErr(ctx context.Context, p naming.Path, nodename string, value instance.MonitorUpdate) (*SetInstanceMonitor, errcontext.ErrReceiver) {
	err := errcontext.New(ctx)
	return &SetInstanceMonitor{Path: p, Node: nodename, Value: value, Err: err}, err
}

func NewSetNodeMonitorWithErr(ctx context.Context, nodename string, value node.MonitorUpdate) (*SetNodeMonitor, errcontext.ErrReceiver) {
	err := errcontext.New(ctx)
	return &SetNodeMonitor{Node: nodename, Value: value, Err: err}, err
}
