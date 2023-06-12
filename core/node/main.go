package node

import (
	"encoding/json"

	"github.com/opensvc/om3/core/instance"
)

type (
	// Node holds a node DataSet.
	Node struct {
		Instance map[string]instance.Instance `json:"instance" yaml:"instance"`
		Monitor  Monitor                      `json:"monitor" yaml:"monitor"`
		Stats    Stats                        `json:"stats" yaml:"stats"`
		Status   Status                       `json:"status" yaml:"status"`
		Os       Os                           `json:"os" yaml:"os"`
		Config   Config                       `json:"config" yaml:"config"`
		//Locks map[string]Lock `json:"locks" yaml:"locks"`
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
