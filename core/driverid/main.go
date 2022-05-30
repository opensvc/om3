// driverid identifies a driver by drivergroup and name.
package driverid

import (
	"fmt"
	"sort"
	"strings"

	"opensvc.com/opensvc/core/drivergroup"
)

type (
	// T is the driverid main struct.
	// It identifies a driver by drivergroup and name.
	T struct {
		Group drivergroup.T
		Name  string
	}
	L []T
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

func (t L) Len() int      { return len(t) }
func (t L) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t L) Less(i, j int) bool {
	return t[i].String() < t[j].String()
}

// Render is a human rendered representation of the driver list
func (t L) Render() string {
	s := ""
	sort.Sort(t)
	for _, did := range t {
		s = s + did.String() + "\n"
	}
	return s
}

func (t T) String() string {
	if t.Name == "" {
		return t.Group.String()
	}
	return fmt.Sprintf("%s.%s", t.Group, t.Name)
}

func (t T) NewGeneric() *T {
	return New(t.Group, "")
}

func Parse(s string) *T {
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

func New(group drivergroup.T, name string) *T {
	if name == "" {
		name, _ = DefaultDriver[group]
	}
	return &T{
		Group: group,
		Name:  name,
	}
}
