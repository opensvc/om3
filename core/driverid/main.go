// driverid identifies a driver by drivergroup and name.
package driverid

import (
	"fmt"
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
)

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
		return &T{
			Group: g,
			Name:  l[1],
		}
	case 1:
		g := drivergroup.New(l[0])
		return &T{
			Group: g,
		}
	default:
		return nil
	}
}

func New(group drivergroup.T, name string) *T {
	return &T{
		Group: group,
		Name:  name,
	}
}
