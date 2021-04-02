package object

import (
	"encoding/json"
	"sort"

	"github.com/guregu/null"
	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/priority"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	// InstanceMonitor describes the in-daemon states of an instance
	InstanceMonitor struct {
		GlobalExpect        string         `json:"global_expect"`
		LocalExpect         string         `json:"local_expect"`
		Status              string         `json:"status"`
		StatusUpdated       timestamp.T    `json:"status_updated"`
		GlobalExpectUpdated timestamp.T    `json:"global_expect_updated"`
		Placement           string         `json:"placement"`
		Restart             map[string]int `json:"restart,omitempty"`
	}

	// InstanceConfig describes a configuration file content checksum,
	// timestamp of last change and the nodes it should be installed on.
	InstanceConfig struct {
		Nodename string   `json:"-"`
		Path     path.T   `json:"-"`
		Checksum string   `json:"csum"`
		Scope    []string `json:"scope"`
		Updated  timestamp.T
	}

	// InstanceStatus describes the instance status.
	InstanceStatus struct {
		Nodename    string                    `json:"-"`
		Path        path.T                    `json:"-"`
		App         string                    `json:"app,omitempty"`
		Avail       status.T                  `json:"avail,omitempty"`
		Constraints bool                      `json:"constraints,omitempty"`
		DRP         bool                      `json:"drp,omitempty"`
		Overall     status.T                  `json:"overall,omitempty"`
		Csum        string                    `json:"csum,omitempty"`
		Env         string                    `json:"env,omitempty"`
		Frozen      timestamp.T               `json:"frozen,omitempty"`
		Kind        kind.T                    `json:"kind"`
		Monitor     InstanceMonitor           `json:"monitor"`
		Optional    status.T                  `json:"optional,omitempty"`
		Orchestrate string                    `json:"orchestrate,omitempty"` // TODO enum
		Topology    string                    `json:"topology,omitempty"`    // TODO enum
		Placement   string                    `json:"placement,omitempty"`   // TODO enum
		Priority    priority.T                `json:"priority,omitempty"`
		Provisioned provisioned.T             `json:"provisioned,omitempty"`
		Preserved   bool                      `json:"preserved,omitempty"`
		Updated     timestamp.T               `json:"updated"`
		FlexTarget  int                       `json:"flex_target,omitempty"`
		FlexMin     int                       `json:"flex_min,omitempty"`
		FlexMax     int                       `json:"flex_max,omitempty"`
		Subsets     map[string]SubsetStatus   `json:"subsets,omitempty"`
		Resources   map[string]ResourceStatus `json:"resources,omitempty"`
		Running     ResourceRunningSet        `json:"running,omitempty"`
		Parents     []path.Relation           `json:"parents,omitempty"`
		Children    []path.Relation           `json:"children,omitempty"`
		Slaves      []path.Relation           `json:"slaves,omitempty"`
		Scale       null.Int                  `json:"scale,omitempty"`
	}

	// ResourceOrder is a sortable list representation of the
	// instance status resources map.
	ResourceOrder []ResourceStatus

	// ResourceRunningSet is the list of resource currently running (sync and task).
	ResourceRunningSet []string

	// TagSet is the list of unique tag names found in the resource definition.
	TagSet []string

	// SubsetStatus describes a resource subset properties.
	SubsetStatus struct {
		Parallel bool `json:"parallel,omitempty"`
	}

	// ResourceStatusMonitor tells the daemon if it should trigger a monitor action
	// when the resource is not up.
	ResourceStatusMonitor bool

	// ResourceStatusDisable hints the resource ignores all state transition actions
	ResourceStatusDisable bool

	// ResourceStatusOptional makes this resource status aggregated into Overall
	// instead of Avail instance status. Errors in optional resource don't stop
	// a state transition action.
	ResourceStatusOptional bool

	// ResourceStatusEncap indicates that the resource is handled by the encapsulated
	// agents, and ignored at the hypervisor level.
	ResourceStatusEncap bool

	// ResourceStatusStandby tells the daemon this resource should always be up,
	// even after a stop state transition action.
	ResourceStatusStandby bool

	// ResourceStatus describes the status of a resource of an instance of an object.
	ResourceStatus struct {
		ResourceID  ResourceID              `json:"-"`
		Label       string                  `json:"label"`
		Log         []string                `json:"log,omitempty"`
		Status      status.T                `json:"status"`
		Type        string                  `json:"type"`
		Provisioned ResourceStatusProvision `json:"provisioned,omitempty"`
		Monitor     ResourceStatusMonitor   `json:"monitor,omitempty"`
		Disable     ResourceStatusDisable   `json:"disable,omitempty"`
		Optional    ResourceStatusOptional  `json:"optional,omitempty"`
		Encap       ResourceStatusEncap     `json:"encap,omitempty"`
		Standby     ResourceStatusStandby   `json:"standby,omitempty"`
		Subset      string                  `json:"subset,omitempty"`
		Info        map[string]interface{}  `json:"info,omitempty"`
		Restart     int                     `json:"restart,omitempty"`
		Tags        TagSet                  `json:"tags,omitempty"`
	}

	// ResourceStatusProvision define if and when the resource became provisioned.
	ResourceStatusProvision struct {
		Mtime timestamp.T   `json:"mtime,omitempty"`
		State provisioned.T `json:"state,omitempty"`
	}
)

// FlagString returns a one character representation of the type instance.
func (t ResourceStatusMonitor) FlagString() string {
	if t {
		return "M"
	}
	return "."
}

// FlagString returns a one character representation of the type instance.
func (t ResourceStatusDisable) FlagString() string {
	if t {
		return "D"
	}
	return "."
}

// FlagString returns a one character representation of the type instance.
func (t ResourceStatusOptional) FlagString() string {
	if t {
		return "O"
	}
	return "."
}

// FlagString returns a one character representation of the type instance.
func (t ResourceStatusEncap) FlagString() string {
	if t {
		return "E"
	}
	return "."
}

// FlagString returns a one character representation of the type instance.
func (t ResourceStatusStandby) FlagString() string {
	if t {
		return "S"
	}
	return "."
}

// Has is true if the rid is found running in the Instance Monitor data sent by the daemon.
func (t ResourceRunningSet) Has(rid string) bool {
	for _, r := range t {
		if r == rid {
			return true
		}
	}
	return false
}

// UnmarshalJSON serializes the type instance as JSON.
func (t *InstanceStatus) UnmarshalJSON(b []byte) error {
	type tempT InstanceStatus
	temp := tempT(InstanceStatus{
		Priority: priority.Default,
	})
	if err := json.Unmarshal(b, &temp); err != nil {
		log.Error().Err(err).Msg("unmarshal InstanceStatus")
		return err
	}
	*t = InstanceStatus(temp)
	return nil
}

//
// SortedResources returns a list of resource identifiers sorted by:
// 1/ driver group
// 2/ subset
// 3/ resource name
//
func (t *InstanceStatus) SortedResources() []ResourceStatus {
	l := make([]ResourceStatus, 0)
	for k, v := range t.Resources {
		v.ResourceID = *NewResourceID(k)
		l = append(l, v)
	}
	sort.Sort(ResourceOrder(l))
	return l
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
