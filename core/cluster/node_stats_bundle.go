package cluster

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
		Time       time.Time              `json:"time" yaml:"time"`
		Collector  ThreadStats            `json:"collector" yaml:"collector"`
		Daemon     ThreadStats            `json:"daemon" yaml:"daemon"`
		DNS        ThreadStats            `json:"dns" yaml:"dns"`
		Scheduler  ThreadStats            `json:"scheduler" yaml:"scheduler"`
		Listener   ThreadStats            `json:"listener" yaml:"listener"`
		Monitor    ThreadStats            `json:"monitor" yaml:"monitor"`
		Heartbeats map[string]ThreadStats `json:"-" yaml:"-"`
		Objects    map[string]ObjectStats `json:"objects" yaml:"objects"`
	}

	// ThreadStats holds a daemon thread system resource usage metrics
	ThreadStats struct {
		CPU     CPUStats `json:"cpu" yaml:"cpu"`
		Mem     MemStats `json:"mem" yaml:"mem"`
		Procs   uint64   `json:"procs" yaml:"procs"`
		Threads uint64   `json:"threads" yaml:"threads"`
	}

	// CPUStats holds CPU resource usage metrics.
	CPUStats struct {
		Time uint64 `json:"time" yaml:"time"`
	}

	// MemStats holds CPU resource usage metrics.
	MemStats struct {
		Total uint64 `json:"total" yaml:"total"`
	}

	// BlkStats holds block devices resource usage metrics.
	BlkStats struct {
		Read      uint64 `json:"r" yaml:"r"`
		ReadByte  uint64 `json:"rb" yaml:"rb"`
		Write     uint64 `json:"w" yaml:"w"`
		WriteByte uint64 `json:"wb" yaml:"wb"`
	}

	// NetStats holds network resource usage metrics.
	NetStats struct {
		Read      uint64 `json:"r" yaml:"r"`
		ReadByte  uint64 `json:"rb" yaml:"rb"`
		Write     uint64 `json:"w" yaml:"w"`
		WriteByte uint64 `json:"wb" yaml:"wb"`
	}

	// ObjectStats holds an object (ie cgroup) system resource usage metrics
	ObjectStats struct {
		Blk       BlkStats  `json:"blk" yaml:"blk"`
		Net       NetStats  `json:"net" yaml:"net"`
		Mem       MemStats  `json:"mem" yaml:"mem"`
		CPU       CPUStats  `json:"cpu" yaml:"cpu"`
		Tasks     uint64    `json:"tasks" yaml:"tasks"`
		CreatedAt time.Time `json:"created_at" yaml:"created_at"`
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
