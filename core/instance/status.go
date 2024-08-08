package instance

import (
	"sort"
	"time"

	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/core/status"
)

type (
	MonitorAction string

	// Status describes the instance status.
	Status struct {
		Avail         status.T           `json:"avail"`
		FrozenAt      time.Time          `json:"frozen_at,omitempty"`
		LastStartedAt time.Time          `json:"last_started_at"`
		Optional      status.T           `json:"optional,omitempty"`
		Overall       status.T           `json:"overall"`
		Provisioned   provisioned.T      `json:"provisioned"`
		Resources     ResourceStatuses   `json:"resources,omitempty"`
		Running       ResourceRunningSet `json:"running,omitempty"`
		UpdatedAt     time.Time          `json:"updated_at"`
	}

	ResourceStatuses map[string]resource.Status

	// ResourceRunningSet is the list of resource currently running (sync and task).
	ResourceRunningSet []string

	// ResourceOrder is a sortable list representation of the
	// instance status resources map.
	ResourceOrder []resource.Status
)

func (m ResourceStatuses) DeepCopy() ResourceStatuses {
	n := make(ResourceStatuses)
	for k, v := range m {
		n[k] = *v.DeepCopy()
	}
	return n
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

// SortedResources returns a list of resource identifiers sorted by:
// 1/ driver group
// 2/ subset
// 3/ resource name
func (t *Status) SortedResources() []resource.Status {
	l := make([]resource.Status, 0)
	for rid, rstat := range t.Resources {
		id, err := resourceid.Parse(rid)
		if err != nil {
			continue
		}
		rstat.ResourceID = id
		l = append(l, rstat)
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
	n := t
	n.Running = append(ResourceRunningSet{}, t.Running...)
	n.Resources = t.Resources.DeepCopy()
	return &n
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

// ResourceFlagsString formats resource flags as a vector of characters.
//
//	R  Running
//	M  Monitored
//	D  Disabled
//	O  Optional
//	E  Encap
//	P  Provisioned
//	S  Standby
func (t Status) ResourceFlagsString(rid resourceid.T, r resource.Status) string {
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

func (t ResourceStatuses) Unstructured() map[string]map[string]any {
	m := make(map[string]map[string]any)
	for k, v := range t {
		m[k] = v.Unstructured()
	}
	return m
}

func (t Status) Unstructured() map[string]any {
	m := map[string]any{
		"avail":           t.Avail,
		"last_started_at": t.LastStartedAt,
		"optional":        t.Optional,
		"overall":         t.Overall,
		"provisioned":     t.Provisioned,
		"running":         t.Running,
		"updated_at":      t.UpdatedAt,
	}
	if len(t.Resources) > 0 {
		m["resources"] = t.Resources.Unstructured()
	}
	if !t.FrozenAt.IsZero() {
		m["frozen_at"] = t.FrozenAt
	}
	return m
}
