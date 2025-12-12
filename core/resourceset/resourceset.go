package resourceset

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/pg"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	T struct {
		Name           string
		SectionName    string
		DriverGroup    driver.Group
		Parallel       bool
		PG             *pg.Config
		ResourceLister ResourceLister

		log *plog.Logger
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

func IsSubsetSection(s string) bool {
	return strings.HasPrefix(s, prefix)
}

func SubsetSectionToName(s string) string {
	if !IsSubsetSection(s) {
		return ""
	}
	return s[len(prefix):]
}

func NewList() L {
	return make([]*T, 0)
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
func (t *T) Log() *plog.Logger {
	if t.log == nil {
		return plog.NewLogger(zerolog.New(nil))
	}
	return t.log
}

// SetLogger configures a logger from a parent logger, adding the "subset" attribute
func (t *T) SetLogger(parent *plog.Logger) {
	if parent == nil {
		return
	}
	prefix := parent.Prefix() + t.Name + ": "
	t.log = parent.Attr("subset", t.Name).WithPrefix(prefix)
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
		s = s + ":" + t.Name
	}
	return s
}

// Resources returns the list of resources handled by the resourceset.
// This function make the resourceset a ResourceLister.
func (t T) Resources() resource.Drivers {
	if t.ResourceLister == nil {
		panic(fmt.Errorf("resourceset has no ResourceLister set"))
	}
	return t.filterResources(t.ResourceLister)
}

func (t T) ReconfigureResource(r resource.Driver) error {
	if t.ResourceLister == nil {
		panic(fmt.Errorf("resourceset has no ResourceLister set"))
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
	pgMgr := pg.FromContext(ctx)
	if pgMgr != nil {
		pgMgr.Register(t.PG)
	}
	if t.Parallel {
		hasHitBarrier, err = t.doParallel(ctx, l, resources, barrier, desc, pgMgr, fn)
	} else {
		hasHitBarrier, err = t.doSerial(ctx, l, resources, barrier, desc, pgMgr, fn)
	}
	return
}

func (t T) delayLinkedResource(r resource.Driver, selectedResources resource.Drivers, desc string) bool {
	if !strings.HasPrefix(desc, "link-") {
		// status, pre-status and linked-* actions don't need delaying
		return false
	}
	if linkToer, ok := r.(resource.LinkToer); ok {
		if name := linkToer.LinkTo(); name != "" && selectedResources.HasRID(name) {
			// will be handled by the targeted LinkNameser resource
			return true
		}
	}
	return false
}

func (t T) doParallel(ctx context.Context, l ResourceLister, resources resource.Drivers, barrier, desc string, pgMgr *pg.Mgr, fn DoFunc) (bool, error) {
	hasHitBarrier := false
	selectedResources := l.Resources()
	q := make(chan result, len(resources))
	defer close(q)
	do := func(q chan<- result, r resource.Driver) {
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
		q <- result{
			Error:    err,
			Resource: r,
		}
	}
	nResources := 0
	for _, r := range resources {
		if hasHitBarrier {
			break
		}
		if t.delayLinkedResource(r, selectedResources, desc) {
			continue
		}
		rid := r.RID()
		if rid == barrier {
			hasHitBarrier = true
		}
		if pgMgr != nil {
			pgMgr.Register(r.GetPG())
		}
		nResources += 1
		go do(q, r)
	}
	var errs error
	for i := 0; i < nResources; i++ {
		res := <-q
		switch {
		case res.Error == nil:
		case errors.Is(res.Error, resource.ErrBarrier):
			hasHitBarrier = true
		case res.Resource.IsOptional():
			res.Resource.Log().Errorf("error from optional resource: %s", res.Error)
		default:
			res.Resource.Log().Errorf("%s", res.Error)
			errs = errors.Join(errs, fmt.Errorf("%s: %w", res.Resource.RID(), res.Error))
		}
	}
	return hasHitBarrier, errs
}

// doSerial executes serially fn on every resource of the resourceset that has no dependency on other resources.
//
// e.g.
//
//	with [ip#1 ip#2(netns=container#1) ip#3]
//	with barrier ip#2
//
//	 does:
//	                   hasHitBarrier
//	                   -------------
//	 ip#1 exec fn()    false
//	 ip#2 skip fn()    false (not toggled because ip#2 exec fn() is delayed after container#1)
//	 ip#3 exec fn()    false
//
//	with [ip#1 ip#2(netns=container#1) ip#3]
//	with barrier ip#1
//
//	 does:
//	                   hasHitBarrier
//	                   -------------
//	 ip#1 exec fn()    true
//	 <break on barrier hit>
func (t T) doSerial(ctx context.Context, l ResourceLister, resources resource.Drivers, barrier, desc string, pgMgr *pg.Mgr, fn DoFunc) (bool, error) {
	hasHitBarrier := false
	selectedResources := l.Resources()
	for _, r := range resources {
		if hasHitBarrier {
			break
		}
		if t.delayLinkedResource(r, selectedResources, desc) {
			continue
		}
		rid := r.RID()
		if rid == barrier {
			hasHitBarrier = true
		}
		if pgMgr != nil {
			pgMgr.Register(r.GetPG())
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
		switch {
		case err == nil:
			continue
		case errors.Is(err, resource.ErrBarrier):
			// linkWrap executed resourceset.L.Do again with a fn that can return ErrBarrier
			return true, nil
		case r.IsOptional():
			r.Log().Warnf("error from optional resource: %s", err)
			continue
		default:
			r.Log().Errorf("%s", err)
			return hasHitBarrier, fmt.Errorf("%s: %w", rid, err)
		}
	}
	return hasHitBarrier, nil
}

func (t L) Reverse() {
	sort.Sort(sort.Reverse(t))
}

// Do executes fn on every resourceset of the actor.
// The barrier can be hit on a resource delayed by a resource link (e.g. ip.cni depending on container.docker)
//
// e.g.
//
//	with resourcesets [[ip#1 ip#2(netns=container#1) ip#3] [container#1 container#2]]
//	with barrier ip#2
//	with action start (i.e. ascending)
//
//	does:
//	                                                hasHitBarrier  fn executed on  comment
//	                                                -------------  --------------  -------
//	  rset[ip#1 ip#2(netns=container#1) ip#3].Do    false          ip#1 ip#3       ip#2 is skipped, depends on container#1
//	  rset[container#1 container#2].Do              true           container#1     then ip#2 via linkWrap() => ErrBarrier
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
			return resource.ErrBarrier
		}
	}
	return nil
}
