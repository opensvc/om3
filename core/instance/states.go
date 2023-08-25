package instance

import (
	"time"

	"github.com/opensvc/om3/core/path"
)

type (
	StatesList []States

	// States groups config and status of the object instance as seen by the daemon.
	States struct {
		Path    path.T  `json:"path" yaml:"path"`
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

func (t StatesList) ByNode() map[string]States {
	m := make(map[string]States)
	for _, s := range t {
		m[s.Node.Name] = s
	}
	return m
}
