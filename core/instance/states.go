package instance

import "time"

type (
	// States groups config and status of the object instance as seen by the daemon.
	States struct {
		Node   Node    `json:"node,omitempty"`
		Config Config  `json:"config,omitempty"`
		Status Status  `json:"status,omitempty"`
		Smon   Monitor `json:"smon,omitempty"`
	}

	// Node contains the node information displayed in print status.
	Node struct {
		Name   string    `json:"name"`
		Frozen time.Time `json:"frozen,omitempty"`
	}
)
