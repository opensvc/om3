// Package driver is the package serving the driver registry.
// A driver is identified by group and name, via the ID type.
package driver

import (
	"fmt"
	"sort"
	"strings"
)

type (
	// ID is the driver main struct.
	// It identifies a driver by Group and name.
	ID struct {
		Group Group  `json:"group"`
		Name  string `json:"name"`
	}
	IDs []ID
)

var (
	DefaultDriver = map[Group]string{
		GroupApp:       "forking",
		GroupContainer: "oci",
		GroupIP:        "host",
		GroupTask:      "host",
		GroupVolume:    "",
		GroupSync:      "rsync",
	}
)

func (t IDs) Len() int      { return len(t) }
func (t IDs) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t IDs) Less(i, j int) bool {
	return t[i].String() < t[j].String()
}

// Render is a human rendered representation of the driver list
func (t IDs) Render() string {
	s := ""
	sort.Sort(t)
	for _, did := range t {
		s = s + did.String() + "\n"
	}
	return s
}

func (t ID) Unstructured() map[string]any {
	return map[string]any{
		"group": t.Group.String(),
		"name":  t.Name,
	}
}

func (t ID) String() string {
	if t.Name == "" {
		return t.Group.String()
	}
	return fmt.Sprintf("%s.%s", t.Group, t.Name)
}

func (t ID) IsEmpty() bool {
	return t.Name == "" && t.Group == 0
}

func (t ID) Cap() string {
	s := t.String()
	if (t.Group & resourceGroups) == t.Group {
		return "drivers.resource." + s
	}
	return "drivers." + s
}

func (t ID) NewGenericID() ID {
	return NewID(t.Group, "")
}

func Parse(s string) ID {
	l := strings.Split(s, ".")
	switch len(l) {
	case 2:
		g := NewGroup(l[0])
		return NewID(g, l[1])
	case 1:
		g := NewGroup(l[0])
		return NewID(g, "")
	default:
		return ID{}
	}
}

func NewID(group Group, name string) ID {
	if name == "" {
		name, _ = DefaultDriver[group]
	}
	return ID{
		Group: group,
		Name:  name,
	}
}
