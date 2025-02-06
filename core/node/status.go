package node

import (
	"time"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/util/san"
)

type (
	Status struct {
		Agent           string                      `json:"agent"`
		API             uint64                      `json:"api"`
		Arbitrators     map[string]ArbitratorStatus `json:"arbitrators"`
		Compat          uint64                      `json:"compat"`
		FrozenAt        time.Time                   `json:"frozen_at"`
		Gen             Gen                         `json:"gen"`
		MinAvailMemPct  uint64                      `json:"min_avail_mem"`
		MinAvailSwapPct uint64                      `json:"min_avail_swap"`
		IsLeader        bool                        `json:"is_leader"`
		Labels          Labels                      `json:"labels"`
	}

	// Instances groups instances configuration digest and status
	Instances struct {
		Config  map[string]instance.Config  `json:"config"`
		Status  map[string]instance.Status  `json:"status"`
		Monitor map[string]instance.Monitor `json:"monitor"`
	}

	// ArbitratorStatus describes the internet name of an arbitrator and
	// if it is join-able.
	ArbitratorStatus struct {
		URL    string   `json:"url"`
		Status status.T `json:"status"`
	}

	// NodesInfo is the dataset exposed via the GET /nodes_info handler,
	// used by nodes to:
	// * expand node selector expressions based on labels
	// * setup clusterwide lun mapping from pools backed by san arrays
	NodesInfo map[string]NodeInfo

	NodeInfo struct {
		Env    string    `json:"env"`
		Labels Labels    `json:"labels"`
		Paths  san.Paths `json:"paths"`

		Lsnr daemonsubsystem.Listener `json:"listener"`
	}
)

func (t Status) IsFrozen() bool {
	return !t.FrozenAt.IsZero()
}

func (t Status) IsUnfrozen() bool {
	return t.FrozenAt.IsZero()
}

func (t *Status) DeepCopy() *Status {
	result := *t
	newArbitrator := make(map[string]ArbitratorStatus)
	for n, v := range t.Arbitrators {
		newArbitrator[n] = v
	}
	result.Arbitrators = newArbitrator

	newGen := make(Gen)
	for n, v := range t.Gen {
		newGen[n] = v
	}
	result.Gen = newGen
	result.Labels = t.Labels.DeepCopy()

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

func (t NodesInfo) Keys() []string {
	l := make([]string, len(t))
	i := 0
	for k := range t {
		l[i] = k
		i++
	}
	return l
}

func (t *Status) Unstructured() map[string]any {
	return map[string]any{
		"agent":          t.Agent,
		"api":            t.API,
		"arbitrators":    t.Arbitrators,
		"compat":         t.Compat,
		"frozen_at":      t.FrozenAt,
		"gen":            t.Gen,
		"min_avail_mem":  t.MinAvailMemPct,
		"min_avail_swap": t.MinAvailSwapPct,
		"is_leader":      t.IsLeader,
		"labels":         t.Labels,
	}
}
