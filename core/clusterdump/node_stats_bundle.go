package clusterdump

import (
	"encoding/json"
	"strings"
	"time"
)

type (
	// Stats is a map of node statistics.
	Stats map[string]NodeStatsBundle

	// NodeStatsBundle embeds all daemon threads and each objet system
	// resource usage metrics.
	NodeStatsBundle struct {
		Time       time.Time              `json:"time"`
		Collector  ThreadStats            `json:"collector"`
		Daemon     ThreadStats            `json:"daemon"`
		DNS        ThreadStats            `json:"dns"`
		Scheduler  ThreadStats            `json:"scheduler"`
		Listener   ThreadStats            `json:"listener"`
		Monitor    ThreadStats            `json:"monitor"`
		Heartbeats map[string]ThreadStats `json:"-"`
		Objects    map[string]ObjectStats `json:"objects"`
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
		Time uint64 `json:"time"`
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
		Blk       BlkStats  `json:"blk"`
		Net       NetStats  `json:"net"`
		Mem       MemStats  `json:"mem"`
		CPU       CPUStats  `json:"cpu"`
		Tasks     uint64    `json:"tasks"`
		CreatedAt time.Time `json:"created_at"`
	}
)

// UnmarshalJSON loads a byte array into a DaemonStatus struct
func (t *NodeStatsBundle) UnmarshalJSON(b []byte) error {
	var (
		m   map[string]interface{}
		ns  NodeStatsBundle
		err error
		tmp []byte
	)
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	ns.Heartbeats = make(map[string]ThreadStats)

	for k, v := range m {
		if tmp, err = json.Marshal(v); err != nil {
			return err
		}
		switch k {
		case "cluster":
			if err := json.Unmarshal(tmp, &ns.Daemon); err != nil {
				return err
			}
		case "monitor":
			if err := json.Unmarshal(tmp, &ns.Monitor); err != nil {
				return err
			}
		case "scheduler":
			if err := json.Unmarshal(tmp, &ns.Scheduler); err != nil {
				return err
			}
		case "collector":
			if err := json.Unmarshal(tmp, &ns.Collector); err != nil {
				return err
			}
		case "dns":
			if err := json.Unmarshal(tmp, &ns.DNS); err != nil {
				return err
			}
		case "pid":
			if err := json.Unmarshal(tmp, &ns.Objects); err != nil {
				return err
			}
		case "listener":
			if err := json.Unmarshal(tmp, &ns.Listener); err != nil {
				return err
			}
		default:
			if strings.HasPrefix(k, "hb#") {
				var hb ThreadStats
				if err := json.Unmarshal(tmp, &hb); err != nil {
					return err
				}

				ns.Heartbeats[k] = hb
			}
		}
	}

	*t = ns
	return nil
}
