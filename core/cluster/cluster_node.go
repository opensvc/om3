package cluster

import (
	"encoding/json"
	"time"

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

	// NodeMonitor describes the in-daemon states of a node
	NodeMonitor struct {
		Status              string    `json:"status"`
		LocalExpect         string    `json:"local_expect"`
		GlobalExpect        string    `json:"global_expect"`
		StatusUpdated       time.Time `json:"status_updated"`
		GlobalExpectUpdated time.Time `json:"global_expect_updated"`
		LocalExpectUpdated  time.Time `json:"local_expect_updated"`
	}

	// NodeStats describes systems (cpu, mem, swap) resource usage of a node
	// and an opensvc-specific score.
	NodeStats struct {
		Load15M      float64 `json:"load_15m"`
		MemAvailPct  uint64  `json:"mem_avail"`
		MemTotalMB   uint64  `json:"mem_total"`
		Score        uint64  `json:"score"`
		SwapAvailPct uint64  `json:"swap_avail"`
		SwapTotalMB  uint64  `json:"swap_total"`
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

func (n *NodeMonitor) DeepCopy() *NodeMonitor {
	var d NodeMonitor
	d = *n
	return &d
}
