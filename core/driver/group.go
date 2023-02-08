package driver

import (
	"bytes"
	"encoding/json"

	"github.com/opensvc/om3/util/xmap"
)

//
// Group groups drivers sharing some properties.
// A resourceset is a collection of resources having the same drivergroup and subset.
//
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
	GroupCertificate
	GroupExpose
	GroupRoute
	GroupVhost
	GroupPool
	GroupNetwork
	GroupHeartbeat
	GroupArray
	GroupSwitch
	GroupStonith
	GroupBackup
)

var (
	resourceGroups = GroupIP | GroupVolume | GroupDisk | GroupFS | GroupShare | GroupContainer | GroupApp | GroupSync | GroupTask | GroupCertificate | GroupExpose | GroupRoute | GroupVhost

	toGroupID = map[string]Group{
		"ip":          GroupIP,
		"volume":      GroupVolume,
		"disk":        GroupDisk,
		"fs":          GroupFS,
		"share":       GroupShare,
		"container":   GroupContainer,
		"app":         GroupApp,
		"sync":        GroupSync,
		"task":        GroupTask,
		"certificate": GroupCertificate,
		"expose":      GroupExpose,
		"route":       GroupRoute,
		"vhost":       GroupVhost,
		"pool":        GroupPool,
		"network":     GroupNetwork,
		"hb":          GroupHeartbeat,
		"array":       GroupArray,
		"switch":      GroupSwitch,
		"stonith":     GroupStonith,
		"backup":      GroupBackup,
	}
	toGroupString = map[Group]string{
		GroupIP:          "ip",
		GroupVolume:      "volume",
		GroupDisk:        "disk",
		GroupFS:          "fs",
		GroupShare:       "share",
		GroupContainer:   "container",
		GroupApp:         "app",
		GroupSync:        "sync",
		GroupTask:        "task",
		GroupCertificate: "certificate",
		GroupExpose:      "expose",
		GroupRoute:       "route",
		GroupVhost:       "vhost",
		GroupPool:        "pool",
		GroupNetwork:     "network",
		GroupHeartbeat:   "hb",
		GroupArray:       "array",
		GroupSwitch:      "switch",
		GroupStonith:     "stonith",
		GroupBackup:      "backup",
	}
)

// Names returns all supported drivergroup names
func GroupNames() []string {
	return xmap.Keys(toGroupID)
}

// New allocates a Group from its string representation.
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

// MarshalJSON marshals the enum as a quoted json string
func (t Group) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(t.String())
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *Group) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	*t = NewGroup(j)
	return nil
}
