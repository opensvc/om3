package resourceset

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/util/pg"
	"github.com/opensvc/om3/util/xerrors"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type (
	T struct {
		Name           string
		SectionName    string
		DriverGroup    driver.Group
		Parallel       bool
		PG             *pg.Config
		ResourceLister ResourceLister

		log *zerolog.Logger
	}

	L []*T

	ResourceLister interface {
		Resources() resource.Drivers
		ReconfigureResource(resource.Driver) error
		IsDesc() bool
	}

	DoFunc func(context.Context, resource.Driver) error

	result struct {
		Error    error
		Resource resource.Driver
	}
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

// Log returns the resource logger
func (t *T) Log() *zerolog.Logger {
	if t.log == nil {
		log := zerolog.New(nil)
		return &log
	}
	return t.log
}

// SetLogger configures a logger from a parent logger, adding the "rs" metadata key
func (t *T) SetLogger(parent *zerolog.Logger) {
	if parent == nil {
		return
	}
	log := parent.With().Str("rs", t.Name).Logger()
	t.log = &log
}

// Generic allocates and initializes a new resourceset for a given
// driver group, and return an error if this name is not valid.
func Generic(driverGroupName string) (*T, error) {
	return Parse(prefix + driverGroupName)
}

// Parse allocates and initializes new resourceset for a given name,
// and return an error if the name is not valid.
func Parse(s string) (*T, error) {
	t := New()
	t.SectionName = s
	if !strings.HasPrefix(s, prefix) {
		return nil, fmt.Errorf("resourceset '%s' is not prefixed with '%s'", s, prefix)
	}
	ss := s[len(prefix):]
	l := strings.SplitN(ss, separator, 2) // ex: subset#disk:g1 => {disk, g1}
	t.DriverGroup = driver.NewGroup(l[0])
	if !t.DriverGroup.IsValid() {
		return nil, fmt.Errorf("resourceset '%s' drivergroup '%s' is not supported", s, l[0])
	}
	if len(l) == 2 {
		t.Name = l[1]
	}
	return t, nil
}

// FormatSectionName returns the resourceset section name for a given
// driver group name and subset name.
func FormatSectionName(driverGroupName, name string) string {
	return prefix + driverGroupName + separator + name
}

func (t T) Fullname() string {
	return t.SectionName[len(prefix):]
}

func (t T) String() string {
	s := prefix + t.DriverGroup.String()
	if t.Name != "" {
		s = s + "." + t.Name
	}
	return s
}

// Resources returns the list of resources handled by the resourceset.
// This function make the resourceset a ResourceLister.
func (t T) Resources() resource.Drivers {
	if t.ResourceLister == nil {
		panic(errors.WithStack(errors.New("resourceset has no ResourceLister set")))
	}
	return t.filterResources(t.ResourceLister)
}

func (t T) ReconfigureResource(r resource.Driver) error {
	if t.ResourceLister == nil {
		panic(errors.WithStack(errors.New("resourceset has no ResourceLister set")))
	}
	return t.ResourceLister.ReconfigureResource(r)
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

func (t T) Do(ctx context.Context, l ResourceLister, barrier, desc string, fn DoFunc) (hasHitBarrier bool, err error) {
	rsetResources := t.Resources()
	resources := l.Resources().Intersection(rsetResources)
	if l.IsDesc() {
		// Align the resources order with the ResourceLister order.
		resources.Reverse()
	}
	resources, hasHitBarrier = resources.Truncate(barrier)
	if pgMgr := pg.FromContext(ctx); pgMgr != nil {
		pgMgr.Register(t.PG)
		for _, r := range resources {
			pgMgr.Register(r.GetPG())
		}
	}
	if t.Parallel {
		err = t.doParallel(ctx, l, resources, desc, fn)
	} else {
		err = t.doSerial(ctx, l, resources, desc, fn)
	}
	return
}

func (t T) doParallel(ctx context.Context, l ResourceLister, resources resource.Drivers, desc string, fn DoFunc) error {
	var err error
	q := make(chan result, len(resources))
	defer close(q)
	do := func(q chan<- result, r resource.Driver) {
		if desc != "" {
			r.Progress(ctx, desc)
		}
		var err error
		c := make(chan error, 1)
		if err = l.ReconfigureResource(r); err == nil {
			c <- fn(ctx, r)
		}
		select {
		case <-ctx.Done():
			err = fmt.Errorf("timeout")
		case err = <-c:
		}
		if desc != "" {
			if err == nil {
				r.Progress(ctx, "")
			} else if r.IsOptional() {
				r.Progress(ctx, rawconfig.Colorize.Warning(err))
			} else {
				r.Progress(ctx, rawconfig.Colorize.Error(err))
			}
		}
		q <- result{
			Error:    err,
			Resource: r,
		}
	}
	for _, r := range resources {
		go do(q, r)
	}
	var errs error
	nResources := len(resources)
	for i := 0; i < nResources; i++ {
		res := <-q
		if res.Resource.IsOptional() {
			continue
		}
		xerrors.Append(err, errors.Wrap(res.Error, res.Resource.RID()))
	}
	return errs
}

func (t T) doSerial(ctx context.Context, l ResourceLister, resources resource.Drivers, desc string, fn DoFunc) error {
	for _, r := range resources {
		t.log.Error().Msgf("xxxx %s %#v", desc, r.RID())
	}
	for _, r := range resources {
		rid := r.RID()
		if desc != "" {
			r.Progress(ctx, desc)
		}
		var err error
		c := make(chan error, 1)
		if err = l.ReconfigureResource(r); err == nil {
			c <- fn(ctx, r)
		}
		select {
		case <-ctx.Done():
			err = fmt.Errorf("timeout")
		case err = <-c:
		}
		if desc != "" {
			if err == nil {
				r.Progress(ctx, "ok")
			} else if r.IsOptional() {
				r.Progress(ctx, rawconfig.Colorize.Warning(err))
			} else {
				r.Progress(ctx, rawconfig.Colorize.Error(err))
			}
		}
		if err == nil {
			continue
		}
		if r.IsOptional() {
			continue
		}
		return errors.Wrap(err, rid)
	}
	return nil
}

func (t L) Reverse() {
	sort.Sort(sort.Reverse(t))
}

func (t L) Do(ctx context.Context, l ResourceLister, barrier, desc string, fn DoFunc) error {
	if l.IsDesc() {
		// Align the resourceset order with the ResourceLister order.
		t.Reverse()
	}
	for _, rset := range t {
		hasHitBarrier, err := rset.Do(ctx, l, barrier, desc, fn)
		if err != nil {
			return err
		}
		if hasHitBarrier {
			break
		}
	}
	return nil
}
