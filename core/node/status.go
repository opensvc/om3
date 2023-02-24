package node

import (
	"time"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/nodesinfo"
	"github.com/opensvc/om3/core/status"
)

type (
	Status struct {
		Agent           string                      `json:"agent"`
		API             uint64                      `json:"api"`
		Arbitrators     map[string]ArbitratorStatus `json:"arbitrators"`
		Compat          uint64                      `json:"compat"`
		Frozen          time.Time                   `json:"frozen"`
		Gen             map[string]uint64           `json:"gen"`
		MinAvailMemPct  uint64                      `json:"min_avail_mem"`
		MinAvailSwapPct uint64                      `json:"min_avail_swap"`
		Speaker         bool                        `json:"speaker"`
		Labels          nodesinfo.Labels            `json:"labels"`
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
		Name   string   `json:"name"`
		Status status.T `json:"status"`
	}
)

func (t Status) IsFrozen() bool {
	return !t.Frozen.IsZero()
}

func (t Status) IsThawed() bool {
	return t.Frozen.IsZero()
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
