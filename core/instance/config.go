package instance

import (
	"time"

	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/priority"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/util/stringslice"
	"github.com/opensvc/om3/util/xmap"
)

type (
	// Config describes a configuration file content checksum,
	// timestamp of last change and the nodes it should be installed on.
	Config struct {
		App              string           `json:"app,omitempty"`
		Checksum         string           `json:"csum"`
		Children         []path.Relation  `json:"children,omitempty"`
		DRP              bool             `json:"drp,omitempty"`
		Env              string           `json:"env,omitempty"`
		FlexMax          int              `json:"flex_max,omitempty"`
		FlexMin          int              `json:"flex_min,omitempty"`
		FlexTarget       int              `json:"flex_target,omitempty"`
		MonitorAction    MonitorAction    `json:"monitor_action,omitempty"`
		PreMonitorAction string           `json:"pre_monitor_action,omitempty"`
		Nodename         string           `json:"-"`
		Orchestrate      string           `json:"orchestrate"`
		Path             path.T           `json:"-"`
		Parents          []path.Relation  `json:"parents,omitempty"`
		PlacementPolicy  placement.Policy `json:"placement_policy"`
		Priority         priority.T       `json:"priority,omitempty"`
		Resources        ResourceConfigs  `json:"resources"`
		Scope            []string         `json:"scope"`
		Subsets          SubsetConfigs    `json:"subsets"`
		Topology         topology.T       `json:"topology"`
		UpdatedAt        time.Time        `json:"updated_at"`
	}
	ResourceConfigs map[string]ResourceConfig
	ResourceConfig  struct {
		IsDisabled   bool           `json:"is_disabled"`
		IsMonitored  bool           `json:"is_monitored"`
		IsStandby    bool           `json:"is_standby"`
		Restart      int            `json:"restart"`
		RestartDelay *time.Duration `json:"restart_delay"`
	}
	SubsetConfig struct {
		Parallel bool `json:"parallel,omitempty"`
	}
	SubsetConfigs map[string]SubsetConfig
)

func (m ResourceConfigs) DeepCopy() ResourceConfigs {
	return xmap.Copy(m)
}

func (m SubsetConfigs) DeepCopy() SubsetConfigs {
	return xmap.Copy(m)
}

func (cfg Config) DeepCopy() *Config {
	newCfg := cfg
	newCfg.Scope = append([]string{}, cfg.Scope...)
	newCfg.Subsets = cfg.Subsets.DeepCopy()
	newCfg.Resources = cfg.Resources.DeepCopy()
	return &newCfg
}

// ConfigEqual returns a boolean reporting whether a == b
//
// Nodename and Path are not compared
func ConfigEqual(a, b *Config) bool {
	if a.UpdatedAt != b.UpdatedAt {
		return false
	}
	if a.Checksum != b.Checksum {
		return false
	}
	if !stringslice.Equal(a.Scope, b.Scope) {
		return false
	}
	return true
}

func (rcfgs ResourceConfigs) Get(rid string) *ResourceConfig {
	for rrid, rcfg := range rcfgs {
		if rrid == rid {
			return &rcfg
		}
	}
	return nil
}
