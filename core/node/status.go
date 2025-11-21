package node

import (
	"time"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/util/label"
)

type (
	Status struct {
		Agent        string                      `json:"agent"`
		API          uint64                      `json:"api"`
		Arbitrators  map[string]ArbitratorStatus `json:"arbitrators"`
		Compat       uint64                      `json:"compat"`
		FrozenAt     time.Time                   `json:"frozen_at"`
		Gen          Gen                         `json:"gen"`
		IsLeader     bool                        `json:"is_leader"`
		IsOverloaded bool                        `json:"is_overloaded"`
		Labels       label.M                     `json:"labels"`
		BootedAt     time.Time                   `json:"booted_at"`
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
		Weight int      `json:"weight"`
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
