package instance

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/placement"
	"github.com/opensvc/om3/core/priority"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/util/stringslice"
)

type (
	Instance struct {
		Config  *Config  `json:"config" yaml:"config"`
		Monitor *Monitor `json:"monitor" yaml:"monitor"`
		Status  *Status  `json:"status" yaml:"status"`
	}

	// Config describes a configuration file content checksum,
	// timestamp of last change and the nodes it should be installed on.
	Config struct {
		Checksum         string                    `json:"csum" yaml:"csum"`
		FlexMax          int                       `json:"flex_max,omitempty" yaml:"flex_max,omitempty"`
		FlexMin          int                       `json:"flex_min,omitempty" yaml:"flex_min,omitempty"`
		FlexTarget       int                       `json:"flex_target,omitempty" yaml:"flex_target,omitempty"`
		MonitorAction    MonitorAction             `json:"monitor_action,omitempty" yaml:"monitor_action,omitempty"`
		PreMonitorAction string                    `json:"pre_monitor_action,omitempty" yaml:"pre_monitor_action,omitempty"`
		Nodename         string                    `json:"-" yaml:"-"`
		Orchestrate      string                    `json:"orchestrate" yaml:"orchestrate"`
		Path             path.T                    `json:"-" yaml:"-"`
		PlacementPolicy  placement.Policy          `json:"placement_policy" yaml:"placement_policy"`
		Priority         priority.T                `json:"priority,omitempty" yaml:"priority,omitempty"`
		Resources        map[string]ResourceConfig `json:"resources" yaml:"resources"`
		Scope            []string                  `json:"scope" yaml:"scope"`
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

	MonitorAction string

	// Status describes the instance status.
	Status struct {
		App         string                   `json:"app,omitempty" yaml:"app,omitempty"`
		Avail       status.T                 `json:"avail" yaml:"avail"`
		Constraints bool                     `json:"constraints,omitempty" yaml:"constraints,omitempty"`
		DRP         bool                     `json:"drp,omitempty" yaml:"drp,omitempty"`
		Overall     status.T                 `json:"overall" yaml:"overall"`
		Csum        string                   `json:"csum,omitempty" yaml:"csum,omitempty"`
		Env         string                   `json:"env,omitempty" yaml:"env,omitempty"`
		FrozenAt    time.Time                `json:"frozen_at,omitempty" yaml:"frozen_at,omitempty"`
		Optional    status.T                 `json:"optional,omitempty" yaml:"optional,omitempty"`
		Provisioned provisioned.T            `json:"provisioned" yaml:"provisioned"`
		Preserved   bool                     `json:"preserved,omitempty" yaml:"preserved,omitempty"`
		UpdatedAt   time.Time                `json:"updated_at" yaml:"updated_at"`
		Subsets     map[string]SubsetStatus  `json:"subsets,omitempty" yaml:"subsets,omitempty"`
		Resources   []resource.ExposedStatus `json:"resources,omitempty" yaml:"resources,omitempty"`
		Running     ResourceRunningSet       `json:"running,omitempty" yaml:"running,omitempty"`
		Parents     []path.Relation          `json:"parents,omitempty" yaml:"parents,omitempty"`
		Children    []path.Relation          `json:"children,omitempty" yaml:"children,omitempty"`
		Slaves      []path.Relation          `json:"slaves,omitempty" yaml:"slaves,omitempty"`
		StatusGroup map[string]string        `json:"status_group,omitempty" yaml:"status_group,omitempty"`
	}

	// ResourceOrder is a sortable list representation of the
	// instance status resources map.
	ResourceOrder []resource.ExposedStatus

	// ResourceRunningSet is the list of resource currently running (sync and task).
	ResourceRunningSet []string

	// SubsetStatus describes a resource subset properties.
	SubsetStatus struct {
		Parallel bool `json:"parallel,omitempty" yaml:"parallel,omitempty"`
	}
)

var (
	MonitorActionCrash      MonitorAction = "crash"
	MonitorActionFreezeStop MonitorAction = "freeze_stop"
	MonitorActionReboot     MonitorAction = "reboot"
	MonitorActionSwitch     MonitorAction = "switch"
)

// Has is true if the rid is found running in the Instance Monitor data sent by the daemon.
func (t ResourceRunningSet) Has(rid string) bool {
	for _, r := range t {
		if r == rid {
			return true
		}
	}
	return false
}

// SortedResources returns a list of resource identifiers sorted by:
// 1/ driver group
// 2/ subset
// 3/ resource name
func (t *Status) SortedResources() []resource.ExposedStatus {
	l := make([]resource.ExposedStatus, 0)
	for _, v := range t.Resources {
		rid, err := resourceid.Parse(v.Rid)
		if err != nil {
			continue
		}
		v.ResourceID = rid
		l = append(l, v)
	}
	sort.Sort(ResourceOrder(l))
	return l
}

func (t Status) IsFrozen() bool {
	return !t.FrozenAt.IsZero()
}

func (t Status) IsThawed() bool {
	return t.FrozenAt.IsZero()
}

func (t Status) DeepCopy() *Status {
	t.Running = append(ResourceRunningSet{}, t.Running...)
	t.Parents = append([]path.Relation{}, t.Parents...)
	t.Children = append([]path.Relation{}, t.Children...)
	t.Slaves = append([]path.Relation{}, t.Slaves...)

	subSets := make(map[string]SubsetStatus)

	for id, v := range t.Subsets {
		subSets[id] = v
	}
	t.Subsets = subSets

	resources := make([]resource.ExposedStatus, 0)
	for _, v := range t.Resources {
		resources = append(resources, *v.DeepCopy())
	}
	t.Resources = resources

	statusGroup := make(map[string]string)
	for id, v := range t.StatusGroup {
		statusGroup[id] = v
	}
	t.StatusGroup = statusGroup

	return &t
}

func (a ResourceOrder) Len() int      { return len(a) }
func (a ResourceOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ResourceOrder) Less(i, j int) bool {
	switch {
	case a[i].ResourceID.DriverGroup() < a[j].ResourceID.DriverGroup():
		return true
	case a[i].ResourceID.DriverGroup() > a[j].ResourceID.DriverGroup():
		return false
	// same driver group
	case a[i].Subset < a[j].Subset:
		return true
	case a[i].Subset > a[j].Subset:
		return false
	// and same subset
	default:
		return a[i].ResourceID.Name < a[j].ResourceID.Name
	}
}

// resourceFlagsString formats resource flags as a vector of characters.
//
//	R  Running
//	M  Monitored
//	D  Disabled
//	O  Optional
//	E  Encap
//	P  Provisioned
//	S  Standby
func (t Status) ResourceFlagsString(rid resourceid.T, r resource.ExposedStatus) string {
	flags := ""

	// Running task or sync
	if t.Running.Has(rid.Name) {
		flags += "R"
	} else {
		flags += "."
	}

	flags += r.Monitor.FlagString()
	flags += r.Disable.FlagString()
	flags += r.Optional.FlagString()
	flags += r.Encap.FlagString()
	flags += r.Provisioned.State.FlagString()
	flags += r.Standby.FlagString()
	return flags
}

func (mon Monitor) ResourceFlagRestartString(rid resourceid.T, r resource.ExposedStatus) string {
	// Restart and retries
	retries := 0
	if rmon, ok := mon.Resources[rid.Name]; ok {
		retries = rmon.Restart.Remaining
	}
	return r.Restart.FlagString(retries)
}

func (cfg Config) DeepCopy() *Config {
	newCfg := cfg
	newCfg.Scope = append([]string{}, cfg.Scope...)
	return &newCfg
}

func (mon Monitor) DeepCopy() *Monitor {
	v := mon
	restart := make(map[string]ResourceMonitor)
	for s, val := range v.Resources {
		restart[s] = val
	}
	v.Resources = restart
	if mon.GlobalExpectOptions != nil {
		switch mon.GlobalExpect {
		case MonitorGlobalExpectPlacedAt:
			b, _ := json.Marshal(mon.GlobalExpectOptions)
			var placedAt MonitorGlobalExpectOptionsPlacedAt
			// TODO Don't ignore following error
			_ = json.Unmarshal(b, &placedAt)
			v.GlobalExpectOptions = placedAt
		// TODO add other cases for globalExpect values that requires GlobalExpectOptions
		default:
			b, _ := json.Marshal(mon.GlobalExpectOptions)
			// TODO Don't ignore following error
			_ = json.Unmarshal(b, &v.GlobalExpectOptions)
		}
	}
	return &v
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
