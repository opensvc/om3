// driver is the package serving the driver registry.
// A driver is identified by group and name, via the ID type.
package driver

import (
	"fmt"
	"sort"
	"strings"

	"opensvc.com/opensvc/core/drivergroup"
)

type (
	// ID is the driver main struct.
	// It identifies a driver by drivergroup and name.
	ID struct {
		Group drivergroup.T
		Name  string
	}
	IDs []ID
)

var (
	DefaultDriver = map[drivergroup.T]string{
		drivergroup.App:       "forking",
		drivergroup.Container: "oci",
		drivergroup.IP:        "host",
		drivergroup.Task:      "host",
		drivergroup.Volume:    "",

		// data resources
		drivergroup.Vhost:       "envoy",
		drivergroup.Certificate: "tls",
		drivergroup.Route:       "envoy",
		drivergroup.Expose:      "envoy",
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

func (t ID) String() string {
	if t.Name == "" {
		return t.Group.String()
	}
	return fmt.Sprintf("%s.%s", t.Group, t.Name)
}

func (t ID) NewGeneric() *ID {
	return New(t.Group, "")
}

func Parse(s string) *ID {
	l := strings.Split(s, ".")
	switch len(l) {
	case 2:
		g := drivergroup.New(l[0])
		return New(g, l[1])
	case 1:
		g := drivergroup.New(l[0])
		return New(g, "")
	default:
		return nil
	}
}

func New(group drivergroup.T, name string) *ID {
	if name == "" {
		name, _ = DefaultDriver[group]
	}
	return &ID{
		Group: group,
		Name:  name,
	}
}
