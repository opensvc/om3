package resourceselector

import (
	"context"
	"strings"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/actionresdeps"
	"opensvc.com/opensvc/core/ordering"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	// Options groups the field of T that get exposed via commandline flags
	Options struct {
		RID    string `flag:"rid"`
		Subset string `flag:"subsets"`
		Tag    string `flag:"tags"`
	}

	// T contains options accepted by all actions manipulating resources
	T struct {
		Options
		order  ordering.T
		lister ResourceLister
		action string
	}

	// ResourceLister is the interface required to list resource.T and see the ordering
	ResourceLister interface {
		Resources() resource.Drivers
		ReconfigureResource(r resource.Driver) error
		IsDesc() bool
	}

	OptionsGetter interface {
		GetOptions() Options
	}

	depser interface {
		GetActionResDeps() *actionresdeps.Store
	}
)

func (t Options) GetOptions() Options {
	return t
}

func WithOptions(o Options) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Options = o
		return nil
	})
}

func WithRID(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.RID = s
		return nil
	})
}

func WithTag(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Tag = s
		return nil
	})
}

func WithSubset(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.Subset = s
		return nil
	})
}

func WithOrder(s ordering.T) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.order = s
		return nil
	})
}

func WithAction(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.action = s
		return nil
	})
}

func New(l ResourceLister, opts ...funcopt.O) *T {
	t := &T{
		lister: l,
	}
	_ = funcopt.Apply(t, opts...)
	return t
}

func (t *T) SetResourceLister(l ResourceLister) {
	t.lister = l
}

func (t T) IsDesc() bool {
	return t.order.IsDesc()
}

func (t T) ReconfigureResource(r resource.Driver) error {
	return t.lister.ReconfigureResource(r)
}

func (t T) Resources() resource.Drivers {
	l := t.lister.Resources()
	if t.RID == "" && t.Tag == "" && t.Subset == "" {
		return l
	}
	var dp *actionresdeps.Store
	if i, ok := t.lister.(depser); ok {
		dp = i.GetActionResDeps()
	}
	fl := make(resource.Drivers, 0)
	f := func(c rune) bool { return c == ',' }
	rids := strings.FieldsFunc(t.RID, f)
	tags := strings.FieldsFunc(t.Tag, f)
	subsets := strings.FieldsFunc(t.Subset, f)
	for _, r := range l {
		for _, e := range rids {
			if r.MatchRID(e) {
				goto add
			}
		}
		for _, e := range subsets {
			if r.MatchSubset(e) {
				goto add
			}
		}
		for _, e := range tags {
			if r.MatchTag(e) {
				goto add
			}
		}
		continue
	add:
		fl = fl.Add(r)
		if dp != nil {
			deps := dp.SelectDependencies(t.action, r.RID())
			for _, rid := range deps {
				if dep := l.GetRID(rid); dep != nil {
					r.Log().Debug().Msgf("add %s to satisfy %s dependency", dep.RID(), t.action)
					fl = fl.Add(dep)
				}
			}
		}
	}
	if t.order == ordering.Desc {
		fl.Reverse()
	} else {
		fl.Sort()
	}
	return fl
}

func (t Options) IsZero() bool {
	switch {
	case t.RID != "":
		return false
	case t.Subset != "":
		return false
	case t.Tag != "":
		return false
	default:
		return true
	}
}

func FromContext(ctx context.Context, l ResourceLister) *T {
	opts := OptionsFromContext(ctx)
	props := actioncontext.Props(ctx)
	return New(
		l,
		WithOptions(opts),
		WithOrder(props.Order),
		WithAction(props.Name),
	)
}

func OptionsFromContext(ctx context.Context) Options {
	if o, ok := actioncontext.Value(ctx).Options.(OptionsGetter); ok {
		return o.GetOptions()
	}
	return Options{}
}
