package instance

import "time"

type (
	// States groups config and status of the object instance as seen by the daemon.
	States struct {
		Node    Node    `json:"node,omitempty" yaml:"node,omitempty"`
		Config  Config  `json:"config,omitempty" yaml:"config,omitempty"`
		Status  Status  `json:"status,omitempty" yaml:"status,omitempty"`
		Monitor Monitor `json:"monitor,omitempty" yaml:"monitor,omitempty"`
	}

	// Node contains the node information displayed in print status.
	Node struct {
		Name     string    `json:"name" yaml:"name"`
		FrozenAt time.Time `json:"frozen_at,omitempty" yaml:"frozen_at,omitempty"`
	}
)
