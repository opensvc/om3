package drivergroup

import (
	"bytes"
	"encoding/json"

	"opensvc.com/opensvc/util/xmap"
)

//
// T groups drivers sharing some properties.
// A resourceset is a collection of resources having the same drivergroup and subset.
//
type T int

const (
	Unknown T = 1 << iota
	IP
	Volume
	Disk
	FS
	Share
	Container
	App
	Sync
	Task
)

var (
	toID = map[string]T{
		"ip":        IP,
		"volume":    Volume,
		"disk":      Disk,
		"fs":        FS,
		"share":     Share,
		"container": Container,
		"app":       App,
		"sync":      Sync,
		"task":      Task,
	}
	toString = map[T]string{
		IP:        "ip",
		Volume:    "volume",
		Disk:      "disk",
		FS:        "fs",
		Share:     "share",
		Container: "container",
		App:       "app",
		Sync:      "sync",
		Task:      "task",
	}
)

// New allocates a drivergroup.T from its string representation.
func New(s string) T {
	if t, ok := toID[s]; ok {
		return t
	}
	return Unknown
}

// IsValid returns true if not Unknown
func (t T) IsValid() bool {
	return t != Unknown
}

// Names returns all supported drivergroup names
func Names() []string {
	return xmap.Keys(toID)
}

func (t T) String() string {
	if s, ok := toString[t]; ok {
		return s
	}
	return "unknown"
}

// MarshalJSON marshals the enum as a quoted json string
func (t T) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(t.String())
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *T) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	*t = New(j)
	return nil
}
