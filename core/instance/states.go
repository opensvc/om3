package instance

import "opensvc.com/opensvc/util/timestamp"

type (
	// States groups config and status of the object instance as seen by the daemon.
	States struct {
		Node   Node   `json:"node,omitempty"`
		Config Config `json:"config,omitempty"`
		Status Status `json:"status,omitempty"`
	}

	// Node contains the node information displayed in print status.
	Node struct {
		Name   string      `json:"name"`
		Frozen timestamp.T `json:"frozen,omitempty"`
	}
)
