package object

import (
	"encoding/json"

	"github.com/rs/zerolog/log"
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
		Path     Path     `json:"-"`
		Checksum string   `json:"csum"`
		Scope    []string `json:"scope"`
		Updated  timestamp.T
	}

	// InstanceStatus describes the instance status.
	InstanceStatus struct {
		Nodename    string                    `json:"-"`
		Path        Path                      `json:"-"`
		App         string                    `json:"app,omitempty"`
		Avail       status.T                  `json:"avail,omitempty"`
		DRP         bool                      `json:"drp,omitempty"`
		Overall     status.T                  `json:"overall,omitempty"`
		Csum        string                    `json:"csum,omitempty"`
		Env         string                    `json:"env,omitempty"`
		Frozen      timestamp.T               `json:"frozen,omitempty"`
		Kind        Kind                      `json:"kind"`
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
		Parents     []RelationPath            `json:"parents,omitempty"`
		Children    []RelationPath            `json:"children,omitempty"`
		Slaves      []RelationPath            `json:"slaves,omitempty"`
	}

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
