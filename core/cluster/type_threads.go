package cluster

import "net"

type (
	// ThreadStatus describes a OpenSVC daemon thread: when the thread
	// was last configured, when it was created, its current state and thread
	// id.
	ThreadStatus struct {
		Configured float64       `json:"configured"`
		Created    float64       `json:"created"`
		State      string        `json:"state"`
		TID        int64         `json:"tid"`
		Alerts     []ThreadAlert `json:"alerts,omitempty"`
	}

	// ThreadAlert describes a message with a severity. Embedded in ThreadStatus
	ThreadAlert struct {
		Message  string `json:"message"`
		Severity string `json:"severity"`
	}

	// ListenerThreadStatus describes the OpenSVC daemon listener thread,
	// which is responsible for serving the API.
	ListenerThreadStatus struct {
		ThreadStatus
		Config ListenerThreadStatusConfig `json:"config"`
	}

	// ListenerThreadStatusConfig holds a summary of the listener configuration
	ListenerThreadStatusConfig struct {
		Addr net.IP `json:"addr"`
		Port int    `json:"port"`
	}

	// CollectorThreadStatus describes the OpenSVC daemon collector thread,
	// which is responsible for communicating with the collector on behalf
	// of the cluster. Only one node runs a collector thread.
	CollectorThreadStatus struct {
		ThreadStatus
	}

	// DNSThreadStatus describes the OpenSVC daemon dns thread, which is
	// responsible for janitoring and serving the cluster DNS zone. This
	// zone is dynamically populated by ip address allocated for the
	// services (frontend and backend).
	DNSThreadStatus struct {
		ThreadStatus
	}

	// HeartbeatThreadStatus describes one OpenSVC daemon heartbeat thread,
	// which is responsible for sending or receiving the node DataSet
	// changes to or from peer nodes.
	HeartbeatThreadStatus struct {
		ThreadStatus
		Peers map[string]HeartbeatPeerStatus `json:"peers"`
	}

	// HeartbeatPeerStatus describes the status of the communication
	// with a specific peer node.
	HeartbeatPeerStatus struct {
		Beating bool    `json:"beating"`
		Last    float64 `json:"last"`
	}

	// SchedulerThreadStatus describes the OpenSVC daemon scheduler thread
	// state, which is responsible for executing node and objects scheduled
	// jobs.
	SchedulerThreadStatus struct {
		ThreadStatus
	}
)
