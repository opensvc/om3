package instance

import (
	"encoding/json"
	"sort"

	"github.com/guregu/null"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/placement"
	"opensvc.com/opensvc/core/priority"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resourceid"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/topology"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	// Monitor describes the in-daemon states of an instance
	Monitor struct {
		GlobalExpect        string                    `json:"global_expect"`
		LocalExpect         string                    `json:"local_expect"`
		Status              string                    `json:"status"`
		StatusUpdated       timestamp.T               `json:"status_updated"`
		GlobalExpectUpdated timestamp.T               `json:"global_expect_updated"`
		Placement           string                    `json:"placement"`
		Restart             map[string]MonitorRestart `json:"restart,omitempty"`
	}

	// MonitorRestart describes the restart states maintained by the daemon
	// for an object instance.
	MonitorRestart struct {
		Retries int         `json:"retries"`
		Updated timestamp.T `json:"updateed"`
	}

	// Config describes a configuration file content checksum,
	// timestamp of last change and the nodes it should be installed on.
	Config struct {
		Nodename string      `json:"-"`
		Path     path.T      `json:"-"`
		Checksum string      `json:"csum"`
		Scope    []string    `json:"scope"`
		Updated  timestamp.T `json:"updated"`
	}

	// Status describes the instance status.
	Status struct {
		Nodename    string                            `json:"-"`
		Path        path.T                            `json:"-"`
		App         string                            `json:"app,omitempty"`
		Avail       status.T                          `json:"avail"`
		Constraints bool                              `json:"constraints,omitempty"`
		DRP         bool                              `json:"drp,omitempty"`
		Overall     status.T                          `json:"overall"`
		Csum        string                            `json:"csum,omitempty"`
		Env         string                            `json:"env,omitempty"`
		Frozen      timestamp.T                       `json:"frozen,omitempty"`
		Kind        kind.T                            `json:"kind"`
		Monitor     Monitor                           `json:"monitor"`
		Optional    status.T                          `json:"optional,omitempty"`
		Orchestrate string                            `json:"orchestrate,omitempty"` // TODO enum
		Topology    topology.T                        `json:"topology,omitempty"`
		Placement   placement.T                       `json:"placement,omitempty"`
		Priority    priority.T                        `json:"priority,omitempty"`
		Provisioned provisioned.T                     `json:"provisioned,omitempty"`
		Preserved   bool                              `json:"preserved,omitempty"`
		Updated     timestamp.T                       `json:"updated"`
		FlexTarget  int                               `json:"flex_target,omitempty"`
		FlexMin     int                               `json:"flex_min,omitempty"`
		FlexMax     int                               `json:"flex_max,omitempty"`
		Subsets     map[string]SubsetStatus           `json:"subsets,omitempty"`
		Resources   map[string]resource.ExposedStatus `json:"resources,omitempty"`
		Running     ResourceRunningSet                `json:"running,omitempty"`
		Parents     []path.Relation                   `json:"parents,omitempty"`
		Children    []path.Relation                   `json:"children,omitempty"`
		Slaves      []path.Relation                   `json:"slaves,omitempty"`
		Scale       null.Int                          `json:"scale,omitempty"`
		StatusGroup map[string]string                 `json:"status_group,omitempty"`
	}

	// ResourceOrder is a sortable list representation of the
	// instance status resources map.
	ResourceOrder []resource.ExposedStatus

	// ResourceRunningSet is the list of resource currently running (sync and task).
	ResourceRunningSet []string

	// SubsetStatus describes a resource subset properties.
	SubsetStatus struct {
		Parallel bool `json:"parallel,omitempty"`
	}
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

// UnmarshalJSON serializes the type instance as JSON.
func (t *Status) UnmarshalJSON(b []byte) error {
	type tempT Status
	temp := tempT(Status{
		Priority: priority.Default,
	})
	if err := json.Unmarshal(b, &temp); err != nil {
		log.Error().Err(err).Str("b", string(b)).Msg("unmarshal instance status")
		return err
	}
	*t = Status(temp)
	return nil
}

//
// SortedResources returns a list of resource identifiers sorted by:
// 1/ driver group
// 2/ subset
// 3/ resource name
//
func (t *Status) SortedResources() []resource.ExposedStatus {
	l := make([]resource.ExposedStatus, 0)
	for k, v := range t.Resources {
		rid, err := resourceid.Parse(k)
		if err != nil {
			continue
		}
		v.ResourceID = rid
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

//
// resourceFlagsString formats resource flags as a vector of characters.
//
//   R  Running
//   M  Monitored
//   D  Disabled
//   O  Optional
//   E  Encap
//   P  Provisioned
//   S  Standby
//
func (t Status) ResourceFlagsString(rid resourceid.T, r resource.ExposedStatus) string {
	flags := ""

	// Running task or sync
	if t.Running.Has(rid.Name) {
		flags += "R"
	} else {
		flags += "."
	}

	// Restart and retries
	retries := 0
	if restart, ok := t.Monitor.Restart[rid.Name]; ok {
		retries = restart.Retries
	}

	flags += r.Monitor.FlagString()
	flags += r.Disable.FlagString()
	flags += r.Optional.FlagString()
	flags += r.Encap.FlagString()
	flags += r.Provisioned.State.FlagString()
	flags += r.Standby.FlagString()
	flags += r.Restart.FlagString(retries)
	return flags
}
