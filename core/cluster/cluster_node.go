package cluster

import (
	"encoding/json"

	"opensvc.com/opensvc/core/instance"
)

type (
	// NodeData holds a node DataSet.
	NodeData struct {
		Instance map[string]instance.Instance `json:"instance"`
		Monitor  NodeMonitor                  `json:"monitor"`
		Stats    NodeStats                    `json:"stats"`
		Status   NodeStatus                   `json:"status"`
		Os       NodeOs                       `json:"os"`
		//Locks map[string]Lock `json:"locks"`
	}
)

func (n *NodeData) DeepCopy() *NodeData {
	b, err := json.Marshal(n)
	if err != nil {
		return &NodeData{}
	}
	nodeStatus := NodeData{}
	if err := json.Unmarshal(b, &nodeStatus); err != nil {
		return &NodeData{}
	}
	return &nodeStatus
}
