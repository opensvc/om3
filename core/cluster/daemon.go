package cluster

import "time"

type (
	// DaemonSubsystemStatus describes a OpenSVC daemon subsystem: when it
	// was last configured, when it was created, its current state and its
	// id.
	DaemonSubsystemStatus struct {
		Id           string        `json:"id" yaml:"id"`
		ConfiguredAt time.Time     `json:"configured_at" yaml:"configured_at"`
		CreatedAt    time.Time     `json:"created_at" yaml:"created_at"`
		State        string        `json:"state" yaml:"state"`
		Alerts       []ThreadAlert `json:"alerts,omitempty" yaml:"alerts,omitempty"`
	}

	// ThreadAlert describes a message with a severity. Embedded in DaemonSubsystemStatus
	ThreadAlert struct {
		Message  string `json:"message" yaml:"message"`
		Severity string `json:"severity" yaml:"severity"`
	}

	// DaemonCollector describes the OpenSVC daemon collector thread,
	// which is responsible for communicating with the collector on behalf
	// of the cluster. Only one node runs a collector thread.
	DaemonCollector struct {
		DaemonSubsystemStatus `yaml:",inline"`
	}

	// DaemonDNS describes the OpenSVC daemon dns thread, which is
	// responsible for janitoring and serving the cluster DNS zone. This
	// zone is dynamically populated by ip address allocated for the
	// services (frontend and backend).
	DaemonDNS struct {
		DaemonSubsystemStatus `yaml:",inline"`
	}

	// HeartbeatStream describes one OpenSVC daemon heartbeat thread,
	// which is responsible for sending or receiving the node DataSet
	// changes to or from peer nodes.
	HeartbeatStream struct {
		DaemonSubsystemStatus `yaml:",inline"`

		// Type is the heartbeat type example: unicast, ...
		Type string `json:"type" yaml:"type"`

		Peers map[string]HeartbeatPeerStatus `json:"peers" yaml:"peers"`
	}

	// HeartbeatPeerStatus describes the status of the communication
	// with a specific peer node.
	HeartbeatPeerStatus struct {
		IsBeating bool      `json:"is_beating" yaml:"is_beating"`
		LastAt    time.Time `json:"last_at" yaml:"last_at"`
	}
)
