package node

import (
	"encoding/json"
	"maps"
	"slices"
	"time"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/util/flatten"
	"github.com/opensvc/om3/v3/util/label"
	"github.com/opensvc/om3/v3/util/xmap"
)

type (
	Config struct {
		Collector              *collector.Config `json:"collector,omitempty"`
		Env                    string            `json:"env"`
		Hooks                  Hooks             `json:"hooks"`
		Labels                 label.M           `json:"labels"`
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

		// Issues are the configuration faults found on this node that
		// a human has to correct, as the cluster configuration has its
		// own. They are found by the node they name, and travel with
		// its configuration so that a peer, om mon and the tui can
		// report them without repeating the search.
		Issues []string `json:"issues"`
	}

	Hooks []Hook

	Hook struct {
		Name    string   `json:"name"`
		Events  []string `json:"events"`
		Command []string `json:"command"`
	}
)

func (cfg *Config) DeepCopy() *Config {
	newCfg := *cfg
	newCfg.Issues = append([]string{}, cfg.Issues...)
	newCfg.Schedules = append([]schedule.Config{}, cfg.Schedules...)
	newCfg.Labels = cfg.Labels.DeepCopy()
	newCfg.Hooks = cfg.Hooks.DeepCopy()
	newCfg.Collector = cfg.Collector.DeepCopy()
	return &newCfg

}

func (c Config) Equal(other Config) bool {
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

	if !slices.Equal(c.Issues, other.Issues) {
		return false
	}
	if !c.Collector.Equal(other.Collector) {
		return false
	}
	if !maps.Equal(c.Labels, other.Labels) {
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

func (t Hooks) DeepCopy() Hooks {
	l := make(Hooks, len(t))
	for i, hook := range t {
		l[i] = *hook.DeepCopy()
	}
	return l
}

func (t *Hook) Equal(o *Hook) bool {
	if t.Name != o.Name {
		return false
	} else if !slices.Equal(t.Events, o.Events) {
		return false
	} else if !slices.Equal(t.Command, o.Command) {
		return false
	}
	return true
}

func (t *Hook) DeepCopy() *Hook {
	n := *t
	n.Events = append([]string{}, t.Events...)
	n.Command = append([]string{}, t.Command...)
	return &n
}

func (t *Hook) Diff(other Hook) string {
	flattenable := func(hook Hook) map[string]any {
		var m map[string]any
		b, _ := json.Marshal(hook)
		json.Unmarshal(b, &m)
		return m
	}
	m1 := flatten.Flatten(flattenable(*t))
	m2 := flatten.Flatten(flattenable(other))
	return xmap.Diff(m1, m2)
}
