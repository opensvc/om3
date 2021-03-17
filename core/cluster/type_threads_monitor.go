package cluster

import (
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
)

type (

	// MonitorThreadStatus describes the OpenSVC daemon monitor thread state,
	// which is responsible for the node DataSets aggregation and decision
	// making.
	MonitorThreadStatus struct {
		ThreadStatus
		Compat   bool                     `json:"compat"`
		Frozen   bool                     `json:"frozen"`
		Nodes    map[string]NodeStatus    `json:"nodes,omitempty"`
		Services map[string]ServiceStatus `json:"services,omitempty"`
	}

	// NodeStatus holds a node DataSet.
	NodeStatus struct {
		Agent           string                      `json:"agent"`
		Speaker         bool                        `json:"speaker"`
		API             uint64                      `json:"api"`
		Arbitrators     map[string]ArbitratorStatus `json:"arbitrators"`
		Compat          uint64                      `json:"compat"`
		Env             string                      `json:"env"`
		Frozen          float64                     `json:"frozen"`
		Gen             map[string]uint64           `json:"gen"`
		Labels          map[string]string           `json:"labels"`
		MinAvailMemPct  uint64                      `json:"min_avail_mem"`
		MinAvailSwapPct uint64                      `json:"min_avail_swap"`
		Monitor         NodeMonitor                 `json:"monitor"`
		Services        NodeServices                `json:"services,omitempty"`
		Stats           NodeStatusStats             `json:"stats"`
		//Locks map[string]Lock `json:"locks"`
	}

	// NodeStatusStats describes systems (cpu, mem, swap) resource usage of a node
	// and a opensvc-specific score.
	NodeStatusStats struct {
		Load15M      float64 `json:"load_15m"`
		MemAvailPct  uint64  `json:"mem_avail"`
		MemTotalMB   uint64  `json:"mem_total"`
		Score        uint    `json:"score"`
		SwapAvailPct uint64  `json:"swap_avail"`
		SwapTotalMB  uint64  `json:"swap_total"`
	}

	// NodeMonitor describes the in-daemon states of a node
	NodeMonitor struct {
		GlobalExpect        string  `json:"global_expect"`
		Status              string  `json:"status"`
		StatusUpdated       float64 `json:"status_updated"`
		GlobalExpectUpdated float64 `json:"global_expect_updated"`
	}

	// NodeServices groups instances configuration digest and status
	NodeServices struct {
		Config map[string]object.InstanceConfigStatus `json:"config"`
		Status map[string]object.InstanceStatus       `json:"status"`
	}

	// ArbitratorStatus describes the internet name of an arbitrator and
	// if it is joinable.
	ArbitratorStatus struct {
		Name   string      `json:"name"`
		Status status.Type `json:"status"`
	}

	// ServiceStatus contains the object states obtained via
	// aggregation of all instances states.
	ServiceStatus struct {
		Avail       status.Type      `json:"avail,omitempty"`
		Overall     status.Type      `json:"overall,omitempty"`
		Frozen      string           `json:"frozen,omitempty"` // TODO enum
		Placement   string           `json:"placement,omitempty"`
		Provisioned provisioned.Type `json:"provisioned,omitempty"`
	}
)
