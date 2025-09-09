package node

import (
	"encoding/json"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
)

type (
	// Node holds a node DataSet.
	Node struct {
		Instance map[string]instance.Instance `json:"instance"`
		Pool     map[string]pool.Status       `json:"pool"`
		Monitor  Monitor                      `json:"monitor"`
		Stats    Stats                        `json:"stats"`
		Status   Status                       `json:"status"`
		Os       Os                           `json:"os"`
		Config   Config                       `json:"config"`

		Daemon daemonsubsystem.Daemon `json:"daemon"`

		//Locks map[string]Lock `json:"locks"`
	}
)

func (n *Node) DeepCopy() *Node {
	b, err := json.Marshal(n)
	if err != nil {
		return &Node{}
	}
	node := Node{}
	if err := json.Unmarshal(b, &node); err != nil {
		return &Node{}
	}
	return &node
}
