package resourceset

import (
	"fmt"
	"strings"

	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/resource"
)

type (
	T struct {
		Name        string
		SectionName string
		DriverGroup drivergroup.T
		Parallel    bool
	}

	L []*T

	DoFunc func(resource.Driver) error
)

const (
	prefix    = "subset#"
	separator = ":"
)

func NewList() L {
	return L(make([]*T, 0))
}

func (t L) Len() int {
	return len(t)
}

func (t L) Less(i, j int) bool {
	switch {
	case t[i].DriverGroup < t[j].DriverGroup:
		return true
	case t[i].DriverGroup > t[j].DriverGroup:
		return false
	}
	return t[i].Name > t[j].Name
}

func (t L) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

// New allocates and initializes a new resourceset
func New() *T {
	return &T{}
}

//
// Generic allocates and initializes a new resourceset for a given
// drivergroup name, and return an error if this name is not valid.
//
func Generic(driverGroupName string) (*T, error) {
	return Parse(prefix + driverGroupName)
}

//
// Parse allocates and initializes new resourceset for a given name,
// and return an error if the name is not valid.
//
func Parse(s string) (*T, error) {
	t := New()
	t.SectionName = s
	if !strings.HasPrefix(s, prefix) {
		return nil, fmt.Errorf("resourceset '%s' is not prefixed with '%s'", s, prefix)
	}
	ss := s[len(prefix):]
	l := strings.SplitN(ss, separator, 2) // ex: subset#disk:g1 => {disk, g1}
	t.DriverGroup = drivergroup.New(l[0])
	if !t.DriverGroup.IsValid() {
		return nil, fmt.Errorf("resourceset '%s' drivergroup '%s' is not supported", s, l[0])
	}
	if len(l) == 2 {
		t.Name = l[1]
	}
	return t, nil
}

//
// FormatSectionName returns the resourceset section name for a given
// drivergroup name and subset name.
//
func FormatSectionName(driverGroupName, name string) string {
	return prefix + driverGroupName + separator + name
}

func (t T) String() string {
	return t.SectionName
}

func (t T) filterResources(resources []resource.Driver) []resource.Driver {
	l := make([]resource.Driver, 0)
	for _, r := range resources {
		if r.ID().DriverGroup() != t.DriverGroup {
			continue
		}
		if r.RSubset() != t.Name {
			continue
		}
		l = append(l, r)
	}
	return l
}

func (t T) Do(resources []resource.Driver, fn DoFunc) error {
	resources = t.filterResources(resources)
	if t.Parallel {
		return t.doParallel(resources, fn)
	}
	return t.doSerial(resources, fn)
}

func (t T) doParallel(resources []resource.Driver, fn DoFunc) error {
	fmt.Println("xx TODO: resourceset do parallel")
	return nil
}

func (t T) doSerial(resources []resource.Driver, fn DoFunc) error {
	for _, r := range resources {
		err := fn(r)
		if err == nil {
			continue
		}
		if r.IsOptional() {
			//fmt.Println("xx ignore err on optional resource", err, r)
			continue
		}
		return err
	}
	return nil
}
