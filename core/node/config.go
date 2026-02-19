package node

import (
	"slices"
	"time"

	"github.com/opensvc/om3/v3/core/schedule"
)

type (
	Config struct {
		Env                    string            `json:"env"`
		MaintenanceGracePeriod time.Duration     `json:"maintenance_grace_period"`
		MaxParallel            int               `json:"max_parallel"`
		MaxKeySize             int64             `json:"max_key_size"`
		MinAvailMemPct         int               `json:"min_avail_mem_pct"`
		MinAvailSwapPct        int               `json:"min_avail_swap_pct"`
		ReadyPeriod            time.Duration     `json:"ready_period"`
		RejoinGracePeriod      time.Duration     `json:"rejoin_grace_period"`
		Schedules              []schedule.Config `json:"schedules"`
		SplitAction            string            `json:"split_action"`
		SSHKey                 string            `json:"sshkey"`
		PRKey                  string            `json:"prkey"`
		Hooks                  []Hook            `json:"hooks"`
	}

	Hook struct {
		Name    string   `json:"name"`
		Events  []string `json:"events"`
		Command []string `json:"command"`

		sig string
	}
)

func (cfg *Config) DeepCopy() *Config {
	newCfg := *cfg
	newCfg.Schedules = append([]schedule.Config{}, cfg.Schedules...)
	return &newCfg

}

func (c Config) Equals(other Config) bool {
	if c.Env != other.Env ||
		c.MaintenanceGracePeriod != other.MaintenanceGracePeriod ||
		c.MaxParallel != other.MaxParallel ||
		c.MaxKeySize != other.MaxKeySize ||
		c.MinAvailMemPct != other.MinAvailMemPct ||
		c.MinAvailSwapPct != other.MinAvailSwapPct ||
		c.ReadyPeriod != other.ReadyPeriod ||
		c.RejoinGracePeriod != other.RejoinGracePeriod ||
		c.SplitAction != other.SplitAction ||
		c.SSHKey != other.SSHKey ||
		c.PRKey != other.PRKey {
		return false
	}

	// Compare Schedules slice
	if len(c.Schedules) != len(other.Schedules) {
		return false
	}
	for i := range c.Schedules {
		if c.Schedules[i] != other.Schedules[i] {
			return false
		}
	}

	// Compare Hook slice
	if len(c.Hooks) != len(other.Hooks) {
		return false
	}
	for i := range c.Hooks {
		if !c.Hooks[i].Equal(&other.Hooks[i]) {
			return false
		}
	}

	return true
}

func (t *Hook) Sig() string {
	return t.sig
}

func (t *Hook) SetSig(sig string) {
	t.sig = sig
}

func (t *Hook) Equal(o *Hook) bool {
	if t.Name != o.Name {
		return false
	} else if t.sig != o.sig {
		return false
	} else if !slices.Equal(t.Events, o.Events) {
		return false
	} else if !slices.Equal(t.Command, o.Command) {
		return false
	}
	return true
}
