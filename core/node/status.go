package node

import (
	"time"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/util/san"
)

type (
	Status struct {
		Agent           string                      `json:"agent" yaml:"agent"`
		API             uint64                      `json:"api" yaml:"api"`
		Arbitrators     map[string]ArbitratorStatus `json:"arbitrators" yaml:"arbitrators"`
		Compat          uint64                      `json:"compat" yaml:"compat"`
		FrozenAt        time.Time                   `json:"frozen_at" yaml:"frozen_at"`
		Gen             map[string]uint64           `json:"gen" yaml:"gen"`
		MinAvailMemPct  uint64                      `json:"min_avail_mem" yaml:"min_avail_mem"`
		MinAvailSwapPct uint64                      `json:"min_avail_swap" yaml:"min_avail_swap"`
		IsSpeaker       bool                        `json:"is_speaker" yaml:"is_speaker"`
		Labels          Labels                      `json:"labels" yaml:"labels"`
	}

	// Instances groups instances configuration digest and status
	Instances struct {
		Config  map[string]instance.Config  `json:"config" yaml:"config"`
		Status  map[string]instance.Status  `json:"status" yaml:"status"`
		Monitor map[string]instance.Monitor `json:"monitor" yaml:"monitor"`
	}

	// ArbitratorStatus describes the internet name of an arbitrator and
	// if it is join-able.
	ArbitratorStatus struct {
		Url    string   `json:"url" yaml:"url"`
		Status status.T `json:"status" yaml:"status"`
	}

	// NodesInfo is the dataset exposed via the GET /nodes_info handler,
	// used by nodes to:
	// * expand node selector expressions based on labels
	// * setup clusterwide lun mapping from pools backed by san arrays
	NodesInfo map[string]NodeInfo

	NodeInfo struct {
		Env    string    `json:"env" yaml:"env"`
		Labels Labels    `json:"labels" yaml:"labels"`
		Paths  san.Paths `json:"paths" yaml:"paths"`
	}
)

func (t Status) IsFrozen() bool {
	return !t.FrozenAt.IsZero()
}

func (t Status) IsThawed() bool {
	return t.FrozenAt.IsZero()
}

func (t *Status) DeepCopy() *Status {
	result := *t
	newArbitrator := make(map[string]ArbitratorStatus)
	for n, v := range t.Arbitrators {
		newArbitrator[n] = v
	}
	result.Arbitrators = newArbitrator

	newGen := make(map[string]uint64)
	for n, v := range t.Gen {
		newGen[n] = v
	}
	result.Gen = newGen
	result.Labels = t.Labels.DeepCopy()

	return &result
}

// GetNodesInfo returns a NodesInfo struct, ie a map of
// a subset of information from the data cache
func GetNodesInfo() *NodesInfo {
	result := make(NodesInfo)
	for _, nodeConfig := range ConfigData.GetAll() {
		name := nodeConfig.Node
		nodeInfo := NodeInfo{Env: nodeConfig.Value.Env}
		if nodeStatus := StatusData.Get(name); nodeStatus != nil {
			nodeInfo.Labels = nodeStatus.Labels.DeepCopy()
		}
		if osPaths := OsPathsData.Get(name); osPaths != nil {
			nodeInfo.Paths = osPaths.DeepCopy()
		}
		result[name] = nodeInfo
	}
	return &result
}

// GetNodesWithAnyPaths return the list of nodes having any of the given paths.
func (t NodesInfo) GetNodesWithAnyPaths(paths san.Paths) []string {
	l := make([]string, 0)
	for nodename, node := range t {
		if paths.HasAnyOf(node.Paths) {
			l = append(l, nodename)
		}
	}
	return l
}
