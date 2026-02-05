package driver

import (
	"github.com/opensvc/om3/v3/util/xmap"
)

// Group groups drivers sharing some properties.
// A resourceset is a collection of resources having the same drivergroup and subset.
type Group int

const (
	GroupUnknown Group = 1 << iota
	GroupIP
	GroupVolume
	GroupDisk
	GroupFS
	GroupShare
	GroupContainer
	GroupApp
	GroupSync
	GroupTask
	GroupPool
	GroupNetwork
	GroupHeartbeat
	GroupArray
	GroupSwitch
	GroupStonith
	GroupBackup
)

var (
	resourceGroups = GroupIP | GroupVolume | GroupDisk | GroupFS | GroupShare | GroupContainer | GroupApp | GroupSync | GroupTask

	toGroupID = map[string]Group{
		"ip":        GroupIP,
		"volume":    GroupVolume,
		"disk":      GroupDisk,
		"fs":        GroupFS,
		"share":     GroupShare,
		"container": GroupContainer,
		"app":       GroupApp,
		"sync":      GroupSync,
		"task":      GroupTask,
		"pool":      GroupPool,
		"network":   GroupNetwork,
		"hb":        GroupHeartbeat,
		"array":     GroupArray,
		"switch":    GroupSwitch,
		"stonith":   GroupStonith,
		"backup":    GroupBackup,
	}
	toGroupString = map[Group]string{
		GroupIP:        "ip",
		GroupVolume:    "volume",
		GroupDisk:      "disk",
		GroupFS:        "fs",
		GroupShare:     "share",
		GroupContainer: "container",
		GroupApp:       "app",
		GroupSync:      "sync",
		GroupTask:      "task",
		GroupPool:      "pool",
		GroupNetwork:   "network",
		GroupHeartbeat: "hb",
		GroupArray:     "array",
		GroupSwitch:    "switch",
		GroupStonith:   "stonith",
		GroupBackup:    "backup",
	}
)

// GroupNames returns all supported drivergroup names
func GroupNames() []string {
	return xmap.Keys(toGroupID)
}

// NewGroup allocates a Group from its string representation.
func NewGroup(s string) Group {
	if t, ok := toGroupID[s]; ok {
		return t
	}
	return GroupUnknown
}

// IsValid returns true if not GroupUnknown
func (t Group) IsValid() bool {
	return t != GroupUnknown
}

// String implements the Stringer interface
func (t Group) String() string {
	if s, ok := toGroupString[t]; ok {
		return s
	}
	return ""
}

// MarshalText marshals the enum as a quoted json string
func (t Group) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

// UnmarshalText unmashals a quoted json string to the enum value
func (t *Group) UnmarshalText(b []byte) error {
	s := string(b)
	*t = NewGroup(s)
	return nil
}
