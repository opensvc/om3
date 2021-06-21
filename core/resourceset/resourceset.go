package resourceset

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/resource"
)

type (
	T struct {
		Name           string
		SectionName    string
		DriverGroup    drivergroup.T
		Parallel       bool
		ResourceLister ResourceLister
	}

	L []*T

	ResourceLister interface {
		Resources() resource.Drivers
		IsDesc() bool
	}

	DoFunc func(context.Context, resource.Driver) error
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
	return t[i].Name < t[j].Name
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

func (t T) Fullname() string {
	return t.SectionName[len(prefix):]
}

func (t T) String() string {
	return t.SectionName
}

//
// Resources returns the list of resources handled by the resourceset.
// This function make the resourceset a ResourceLister.
//
func (t T) Resources() resource.Drivers {
	if t.ResourceLister == nil {
		panic(errors.WithStack(errors.New("resourceset has no ResourceLister set")))
	}
	return t.filterResources(t.ResourceLister)
}

func (t T) filterResources(resourceLister ResourceLister) resource.Drivers {
	l := make(resource.Drivers, 0)
	for _, r := range resourceLister.Resources() {
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

func (t T) Do(ctx context.Context, resourceLister ResourceLister, barrier string, fn DoFunc) (hitBarrier bool, err error) {
	rsetResources := t.Resources()
	resources := resourceLister.Resources().Intersection(rsetResources)
	if barrier != "" && resources.HasRID(barrier) {
		hitBarrier = true
		resources = resources.Truncate(barrier)
	}
	if t.Parallel {
		err = t.doParallel(ctx, resources, fn)
	} else {
		err = t.doSerial(ctx, resources, fn)
	}
	return
}

type result struct {
	Error    error
	Resource resource.Driver
}

func (t T) doParallel(ctx context.Context, resources resource.Drivers, fn DoFunc) error {
	var err error
	q := make(chan result, len(resources))
	defer close(q)
	do := func(q chan<- result, r resource.Driver) {
		q <- result{
			Error:    fn(ctx, r),
			Resource: r,
		}
	}
	for _, r := range resources {
		go do(q, r)
	}
	for i := 0; i < len(resources); i++ {
		res := <-q
		if res.Resource.IsOptional() {
			continue
		}
		err = res.Error
	}
	return err
}

func (t T) doSerial(ctx context.Context, resources resource.Drivers, fn DoFunc) error {
	for _, r := range resources {
		err := fn(ctx, r)
		if err == nil {
			continue
		}
		if r.IsOptional() {
			continue
		}
		return err
	}
	return nil
}

func (t L) Reverse() {
	sort.Sort(sort.Reverse(t))
}

func (t L) Do(ctx context.Context, resourceLister ResourceLister, barrier string, fn DoFunc) error {
	if resourceLister.IsDesc() {
		// Align the resourceset order with the ResourceLister order.
		t.Reverse()
	}
	for _, rset := range t {
		hitBarrier, err := rset.Do(ctx, resourceLister, barrier, fn)
		if err != nil {
			return err
		}
		if hitBarrier {
			break
		}
	}
	return nil
}
