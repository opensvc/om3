package instance

import (
	"time"

	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/priority"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/util/stringslice"
)

type (
	// Config describes a configuration file content checksum,
	// timestamp of last change and the nodes it should be installed on.
	Config struct {
		App              string                    `json:"app,omitempty" yaml:"app,omitempty"`
		Checksum         string                    `json:"csum" yaml:"csum"`
		Children         []path.Relation           `json:"children" yaml:"children"`
		DRP              bool                      `json:"drp,omitempty" yaml:"drp,omitempty"`
		Env              string                    `json:"env,omitempty" yaml:"env,omitempty"`
		FlexMax          int                       `json:"flex_max,omitempty" yaml:"flex_max,omitempty"`
		FlexMin          int                       `json:"flex_min,omitempty" yaml:"flex_min,omitempty"`
		FlexTarget       int                       `json:"flex_target,omitempty" yaml:"flex_target,omitempty"`
		MonitorAction    MonitorAction             `json:"monitor_action,omitempty" yaml:"monitor_action,omitempty"`
		PreMonitorAction string                    `json:"pre_monitor_action,omitempty" yaml:"pre_monitor_action,omitempty"`
		Nodename         string                    `json:"-" yaml:"-"`
		Orchestrate      string                    `json:"orchestrate" yaml:"orchestrate"`
		Path             path.T                    `json:"-" yaml:"-"`
		Parents          []path.Relation           `json:"parents" yaml:"parents"`
		PlacementPolicy  placement.Policy          `json:"placement_policy" yaml:"placement_policy"`
		Priority         priority.T                `json:"priority,omitempty" yaml:"priority,omitempty"`
		Resources        map[string]ResourceConfig `json:"resources" yaml:"resources"`
		Scope            []string                  `json:"scope" yaml:"scope"`
		Subsets          map[string]SubsetConfig   `json:"subsets" yaml:"subsets"`
		Topology         topology.T                `json:"topology" yaml:"topology"`
		UpdatedAt        time.Time                 `json:"updated_at" yaml:"updated_at"`
	}
	ResourceConfig struct {
		IsDisabled   bool           `json:"is_disabled" yaml:"is_disabled"`
		IsMonitored  bool           `json:"is_monitored" yaml:"is_monitored"`
		IsStandby    bool           `json:"is_standby" yaml:"is_standby"`
		Restart      int            `json:"restart" yaml:"restart"`
		RestartDelay *time.Duration `json:"restart_delay" yaml:"restart_delay"`
	}
	SubsetConfig struct {
		Parallel bool `json:"parallel,omitempty" yaml:"parallel,omitempty"`
	}
)

func (cfg Config) DeepCopy() *Config {
	resources := make(map[string]ResourceConfig)
	subSets := make(map[string]SubsetConfig)

	for id, v := range cfg.Resources {
		resources[id] = v
	}
	for id, v := range cfg.Subsets {
		subSets[id] = v
	}

	newCfg := cfg
	newCfg.Scope = append([]string{}, cfg.Scope...)
	newCfg.Subsets = subSets
	newCfg.Resources = resources

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
