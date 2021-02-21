package cluster

import (
	"encoding/json"
	"strings"
)

type (
	// Stats is a map of node statistics.
	Stats map[string]NodeStats

	// NodeStats embeds all daemon threads and each objet system
	// resource usage metrics.
	NodeStats struct {
		Timestamp  float64                `json:"timestamp"`
		Collector  ThreadStats            `json:"collector"`
		Daemon     ThreadStats            `json:"daemon"`
		DNS        ThreadStats            `json:"dns"`
		Scheduler  ThreadStats            `json:"scheduler"`
		Listener   ThreadStats            `json:"listener"`
		Monitor    ThreadStats            `json:"monitor"`
		Heartbeats map[string]ThreadStats `json:"-"`
		Services   map[string]ObjectStats `json:"services"`
	}

	// ThreadStats holds a daemon thread system resource usage metrics
	ThreadStats struct {
		CPU     CPUStats `json:"cpu"`
		Mem     MemStats `json:"mem"`
		Procs   uint64   `json:"procs"`
		Threads uint64   `json:"threads"`
	}

	// CPUStats holds CPU resource usage metrics.
	CPUStats struct {
		Time float64 `json:"time"`
	}

	// MemStats holds CPU resource usage metrics.
	MemStats struct {
		Total uint64 `json:"total"`
	}

	// BlkStats holds block devices resource usage metrics.
	BlkStats struct {
		Read      uint64 `json:"r"`
		ReadByte  uint64 `json:"rb"`
		Write     uint64 `json:"w"`
		WriteByte uint64 `json:"wb"`
	}

	// NetStats holds network resource usage metrics.
	NetStats struct {
		Read      uint64 `json:"r"`
		ReadByte  uint64 `json:"rb"`
		Write     uint64 `json:"w"`
		WriteByte uint64 `json:"wb"`
	}

	// ObjectStats holds an object (ie cgroup) system resource usage metrics
	ObjectStats struct {
		Blk     BlkStats `json:"blk"`
		Net     NetStats `json:"net"`
		Mem     MemStats `json:"mem"`
		CPU     CPUStats `json:"cpu"`
		Tasks   uint64   `json:"tasks"`
		Created float64  `json:"created"`
	}
)

// UnmarshalJSON loads a byte array into a DaemonStatus struct
func (t *NodeStats) UnmarshalJSON(b []byte) error {
	var m map[string]interface{}
	err := json.Unmarshal(b, &m)
	if err != nil {
		return err
	}
	var ns NodeStats
	var tmp []byte
	ns.Heartbeats = make(map[string]ThreadStats)

	for k, v := range m {
		tmp, err = json.Marshal(v)
		switch k {
		case "cluster":
			json.Unmarshal(tmp, &ns.Daemon)
		case "monitor":
			json.Unmarshal(tmp, &ns.Monitor)
		case "scheduler":
			json.Unmarshal(tmp, &ns.Scheduler)
		case "collector":
			json.Unmarshal(tmp, &ns.Collector)
		case "dns":
			json.Unmarshal(tmp, &ns.DNS)
		case "pid":
			json.Unmarshal(tmp, &ns.Services)
		case "listener":
			json.Unmarshal(tmp, &ns.Listener)
		default:
			if strings.HasPrefix(k, "hb#") {
				var hb ThreadStats
				json.Unmarshal(tmp, &hb)
				ns.Heartbeats[k] = hb
			}
		}
	}

	*t = ns
	return nil
}
