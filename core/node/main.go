package node

import (
	"encoding/json"

	"opensvc.com/opensvc/core/instance"
)

type (
	// Node holds a node DataSet.
	Node struct {
		Instance map[string]instance.Instance `json:"instance"`
		Monitor  Monitor                      `json:"monitor"`
		Stats    Stats                        `json:"stats"`
		Status   Status                       `json:"status"`
		Os       Os                           `json:"os"`
		Config   Config                       `json:"config"`
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
