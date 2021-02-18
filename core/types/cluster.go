package types

import (
	"encoding/json"
	"net"
	"strings"

	"opensvc.com/opensvc/core/objects/kinds"
	"opensvc.com/opensvc/core/status"
)

type (
	// DaemonStatus describes the full Cluster state.
	DaemonStatus struct {
		Cluster    ClusterInfo                      `json:"cluster"`
		Collector  CollectorThreadStatus            `json:"collector"`
		DNS        DNSThreadStatus                  `json:"dns"`
		Scheduler  SchedulerThreadStatus            `json:"scheduler"`
		Listener   ListenerThreadStatus             `json:"listener"`
		Monitor    MonitorThreadStatus              `json:"monitor"`
		Heartbeats map[string]HeartbeatThreadStatus `json:"-"`
	}

	// ClusterInfo decribes the cluster id, name and nodes
	// The cluster name is used as the right most part of cluster dns
	// names.
	ClusterInfo struct {
		ID    string   `json:"id"`
		Name  string   `json:"name"`
		Nodes []string `json:"nodes"`
	}

	// ThreadStatus describes a OpenSVC daemon thread: when the thread
	// was last configured, when it was created, its current state and thread
	// id.
	ThreadStatus struct {
		Configured float64       `json:"configured"`
		Created    float64       `json:"created"`
		State      string        `json:"state"`
		TID        int64         `json:"tid"`
		Alerts     []ThreadAlert `json:"alerts,omitempty"`
	}

	// ThreadAlert describes a message with a severity. Embedded in ThreadStatus
	ThreadAlert struct {
		Message  string `json:"message"`
		Severity string `json:"severity"`
	}

	// ListenerThreadStatus describes the OpenSVC daemon listener thread,
	// which is responsible for serving the API.
	ListenerThreadStatus struct {
		ThreadStatus
		Config ListenerThreadStatusConfig `json:"config"`
	}

	// ListenerThreadStatusConfig holds a summary of the listener configuration
	ListenerThreadStatusConfig struct {
		Addr net.IP `json:"addr"`
		Port int
	}

	// CollectorThreadStatus describes the OpenSVC daemon collector thread,
	// which is responsible for communicating with the collector on behalf
	// of the cluster. Only one node runs a collector thread.
	CollectorThreadStatus struct {
		ThreadStatus
	}

	// DNSThreadStatus describes the OpenSVC daemon dns thread, which is
	// responsible for janitoring and serving the cluster DNS zone. This
	// zone is dynamically populated by ip address allocated for the
	// services (frontend and backend).
	DNSThreadStatus struct {
		ThreadStatus
	}

	// HeartbeatThreadStatus describes one OpenSVC daemon heartbeat thread,
	// which is responsible for sending or receiving the node DataSet
	// changes to or from peer nodes.
	HeartbeatThreadStatus struct {
		ThreadStatus
		Peers map[string]HeartbeatPeerStatus `json:"peers"`
	}

	// HeartbeatPeerStatus describes the status of the communication
	// with a specific peer node.
	HeartbeatPeerStatus struct {
		Beating bool    `json:"beating"`
		Last    float64 `json:"last"`
	}

	// SchedulerThreadStatus describes the OpenSVC daemon scheduler thread
	// state, which is responsible for executing node and objects scheduled
	// jobs.
	SchedulerThreadStatus struct {
		ThreadStatus
	}

	// MonitorThreadStatus describes the OpenSVC daemon monitor thread state,
	// which is responsible for the node DataSets aggregation and decision
	// making.
	MonitorThreadStatus struct {
		ThreadStatus
		Compat   bool                     `json:"compat"`
		Frozen   bool                     `json:"frozen"`
		Nodes    map[string]NodeStatus    `json:"nodes"`
		Services map[string]ServiceStatus `json:"services"`
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
		Services        NodeServices                `json:"services"`
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

	// InstanceMonitor describes the in-daemon states of an instance
	InstanceMonitor struct {
		GlobalExpect        string  `json:"global_expect"`
		LocalExpect         string  `json:"local_expect"`
		Status              string  `json:"status"`
		StatusUpdated       float64 `json:"status_updated"`
		GlobalExpectUpdated float64 `json:"global_expect_updated"`
		Placement           string  `json:"placement"`
	}

	// NodeServices groups instances configuration digest and status
	NodeServices struct {
		Config map[string]InstanceConfigStatus `json:"config"`
		Status map[string]InstanceStatus       `json:"status"`
	}

	// InstanceConfigStatus describes a configuration file content checksum,
	// timestamp of last change and the nodes it should be installed on.
	InstanceConfigStatus struct {
		Checksum string   `json:"csum"`
		Scope    []string `json:"scope"`
		Updated  float64
	}

	// InstanceStatus describes the instance status.
	InstanceStatus struct {
		App     string          `json:"app"`
		Avail   status.Type     `json:"avail"`
		Overall status.Type     `json:"overall"`
		Csum    string          `json:"csum"`
		Env     string          `json:"env"`
		Frozen  float64         `json:"frozen"`
		Kind    kinds.Type      `json:"kind"`
		Monitor InstanceMonitor `json:"monitor"`
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
		Avail       status.Type `json:"avail"`
		Overall     status.Type `json:"overall"`
		Frozen      float64     `json:"frozen"`
		Placement   string      `json:"placement"`
		Provisioned bool        `json:"provisioned"`
	}
)

// UnmarshalJSON loads a byte array into a DaemonStatus struct
func (t *DaemonStatus) UnmarshalJSON(b []byte) error {
	var m map[string]interface{}
	err := json.Unmarshal(b, &m)
	if err != nil {
		return err
	}
	var ds DaemonStatus
	var tmp []byte
	ds.Heartbeats = make(map[string]HeartbeatThreadStatus)

	for k, v := range m {
		tmp, err = json.Marshal(v)
		switch k {
		case "cluster":
			json.Unmarshal(tmp, &ds.Cluster)
		case "monitor":
			json.Unmarshal(tmp, &ds.Monitor)
		case "scheduler":
			json.Unmarshal(tmp, &ds.Scheduler)
		case "collector":
			json.Unmarshal(tmp, &ds.Collector)
		case "dns":
			json.Unmarshal(tmp, &ds.DNS)
		case "listener":
			json.Unmarshal(tmp, &ds.Listener)
		default:
			if strings.HasPrefix(k, "hb#") {
				var hb HeartbeatThreadStatus
				json.Unmarshal(tmp, &hb)
				ds.Heartbeats[k] = hb
			}
		}
	}

	*t = ds
	return nil
}
