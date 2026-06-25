package node

import (
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/status"
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
		BootedAt     time.Time                   `json:"booted_at"`

		// LeftAt is the last time the nmon advanced the last_shutdown file,
		// i.e. the last time it proved it was alive and rejoined.
		LeftAt time.Time `json:"left_at"`

		// RejoinedAt is the time nmon state transitioned from rejoin to idle.
		// This happens either when the rejoin_grace_period expires or when
		// we received data from all peers.
		RejoinedAt time.Time `json:"rejoined_at"`
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

	return &result
}
