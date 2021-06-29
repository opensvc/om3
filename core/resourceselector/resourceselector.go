package resourceselector

import (
	"context"
	"strings"

	"opensvc.com/opensvc/core/actioncontext"
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
	}

	// ResourceLister is the interface required to list resource.T and see the ordering
	ResourceLister interface {
		Resources() resource.Drivers
		IsDesc() bool
	}

	OptionsGetter interface {
		GetOptions() Options
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

func (t T) Resources() resource.Drivers {
	l := t.lister.Resources()
	if t.order == ordering.Desc {
		l.Reverse()
	} else {
		l.Sort()
	}
	if t.RID == "" && t.Tag == "" && t.Subset == "" {
		return l
	}
	fl := make([]resource.Driver, 0)
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
		fl = append(fl, r)
	}
	return fl
}

func (t Options) IsZero() bool {
	switch {
	case t.RID != "":
		return true
	case t.Subset != "":
		return true
	case t.Tag != "":
		return true
	default:
		return false
	}
}

func FromContext(ctx context.Context, l ResourceLister) *T {
	opts := OptionsFromContext(ctx)
	order := actioncontext.Props(ctx).Order
	return New(
		l,
		WithOptions(opts),
		WithOrder(order),
	)
}

func OptionsFromContext(ctx context.Context) Options {
	if o, ok := actioncontext.Value(ctx).Options.(OptionsGetter); ok {
		return o.GetOptions()
	}
	return Options{}
}
