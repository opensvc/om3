package instance

import (
	"time"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/priority"
	"github.com/opensvc/om3/core/schedule"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/util/stringslice"
	"github.com/opensvc/om3/util/xmap"
)

type (
	// Config describes a configuration file content checksum,
	// timestamp of last change and the nodes it should be installed on.
	Config struct {
		Path      naming.Path `json:"-"`
		Checksum  string      `json:"csum"`
		Priority  priority.T  `json:"priority"`
		Scope     []string    `json:"scope"`
		UpdatedAt time.Time   `json:"updated_at"`

		*ActorConfig
		*VolConfig
	}

	ActorConfig struct {
		App              string            `json:"app,omitempty"`
		Children         naming.Relations  `json:"children,omitempty"`
		DRP              bool              `json:"drp,omitempty"`
		Env              string            `json:"env,omitempty"`
		MonitorAction    []MonitorAction   `json:"monitor_action,omitempty"`
		PreMonitorAction string            `json:"pre_monitor_action,omitempty"`
		Orchestrate      string            `json:"orchestrate"`
		Parents          naming.Relations  `json:"parents,omitempty"`
		PlacementPolicy  placement.Policy  `json:"placement_policy"`
		Resources        ResourceConfigs   `json:"resources"`
		Schedules        []schedule.Config `json:"schedules"`
		Stonith          bool              `json:"stonith"`
		Subsets          SubsetConfigs     `json:"subsets"`
		Topology         topology.T        `json:"topology,omitempty"`
		Flex             *FlexConfig       `json:"flex,omitempty"`

		// IsDisabled is true when DEFAULT.disable is true
		IsDisabled bool `json:"is_disabled"`
	}

	FlexConfig struct {
		Max    int `json:"max,omitempty"`
		Min    int `json:"min,omitempty"`
		Target int `json:"target,omitempty"`
	}

	VolConfig struct {
		Pool string `json:"pool"`
		Size int64  `json:"size"`
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

func (cfg *Config) DeepCopy() *Config {
	if cfg == nil {
		return nil
	}
	newCfg := *cfg
	newCfg.Scope = append([]string{}, cfg.Scope...)
	newCfg.ActorConfig = cfg.ActorConfig.DeepCopy()
	newCfg.VolConfig = cfg.VolConfig.DeepCopy()
	return &newCfg
}

func (cfg *VolConfig) DeepCopy() *VolConfig {
	if cfg == nil {
		return nil
	}
	newCfg := *cfg
	return &newCfg
}

func (cfg *ActorConfig) DeepCopy() *ActorConfig {
	if cfg == nil {
		return nil
	}
	newCfg := *cfg
	newCfg.Subsets = cfg.Subsets.DeepCopy()
	newCfg.Resources = cfg.Resources.DeepCopy()
	newCfg.Schedules = append([]schedule.Config{}, cfg.Schedules...)
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

func (t Config) Unstructured() map[string]any {
	m := map[string]any{
		"csum":       t.Checksum,
		"priority":   t.Priority,
		"scope":      t.Scope,
		"updated_at": t.UpdatedAt,
	}
	if t.ActorConfig != nil {
		m["app"] = t.App
		m["children"] = t.Children
		m["drp"] = t.DRP
		m["env"] = t.Env
		m["is_disabled"] = t.IsDisabled
		m["monitor_action"] = t.MonitorAction
		m["pre_monitor_action"] = t.PreMonitorAction
		m["orchestrate"] = t.Orchestrate
		m["parents"] = t.Parents
		m["placement_policy"] = t.PlacementPolicy
		m["resources"] = t.Resources.Unstructured()
		m["subsets"] = t.Subsets.Unstructured()
		m["topology"] = t.Topology
		m["stonith"] = t.Stonith
		if t.ActorConfig.Flex != nil {
			m["max"] = t.ActorConfig.Flex.Max
			m["min"] = t.ActorConfig.Flex.Min
			m["target"] = t.ActorConfig.Flex.Target
		}
	}
	if t.VolConfig != nil {
		m["pool"] = t.VolConfig.Pool
		m["size"] = t.VolConfig.Size
	}
	return m
}

func (t ResourceConfig) Unstructured() map[string]any {
	m := map[string]any{
		"is_disabled":   t.IsDisabled,
		"is_monitored":  t.IsMonitored,
		"is_standby":    t.IsStandby,
		"restart":       t.Restart,
		"restart_delay": t.RestartDelay,
	}
	return m
}

func (t ResourceConfigs) Unstructured() map[string]map[string]any {
	m := make(map[string]map[string]any)
	for k, v := range t {
		m[k] = v.Unstructured()
	}
	return m
}

func (t SubsetConfigs) Unstructured() map[string]map[string]any {
	m := make(map[string]map[string]any)
	for k, v := range t {
		m[k] = v.Unstructured()
	}
	return m
}

func (t SubsetConfig) Unstructured() map[string]any {
	return map[string]any{
		"parallel": t.Parallel,
	}
}
