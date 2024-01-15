package cluster

import "time"

type (
	// DaemonSubsystemStatus describes a OpenSVC daemon subsystem: when it
	// was last configured, when it was created, its current state and its
	// id.
	DaemonSubsystemStatus struct {
		ID           string        `json:"id"`
		ConfiguredAt time.Time     `json:"configured_at"`
		CreatedAt    time.Time     `json:"created_at"`
		State        string        `json:"state"`
		Alerts       []ThreadAlert `json:"alerts,omitempty"`
	}

	// ThreadAlert describes a message with a severity. Embedded in DaemonSubsystemStatus
	ThreadAlert struct {
		Message  string `json:"message"`
		Severity string `json:"severity"`
	}

	// DaemonCollector describes the OpenSVC daemon collector thread,
	// which is responsible for communicating with the collector on behalf
	// of the cluster. Only one node runs a collector thread.
	DaemonCollector struct {
		DaemonSubsystemStatus
	}

	// DaemonDNS describes the OpenSVC daemon dns thread, which is
	// responsible for janitoring and serving the cluster DNS zone. This
	// zone is dynamically populated by ip address allocated for the
	// services (frontend and backend).
	DaemonDNS struct {
		DaemonSubsystemStatus
	}

	// HeartbeatStream describes one OpenSVC daemon heartbeat thread,
	// which is responsible for sending or receiving the node DataSet
	// changes to or from peer nodes.
	HeartbeatStream struct {
		DaemonSubsystemStatus

		// Type is the heartbeat type example: unicast, ...
		Type string `json:"type"`

		Peers map[string]HeartbeatPeerStatus `json:"peers"`
	}

	// HeartbeatPeerStatus describes the status of the communication
	// with a specific peer node.
	HeartbeatPeerStatus struct {
		IsBeating bool      `json:"is_beating"`
		LastAt    time.Time `json:"last_at"`
	}
)
