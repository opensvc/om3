package resourceselector

import (
	"strings"

	"opensvc.com/opensvc/core/ordering"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		RID    string
		Tag    string
		Subset string
		Order  ordering.T

		lister ResourceLister
	}

	ResourceLister interface {
		Resources() resource.Drivers
		IsDesc() bool
	}
)

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
		t.Order = s
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

func (t T) IsDesc() bool {
	return t.Order.IsDesc()
}

func (t T) Resources() resource.Drivers {
	l := t.lister.Resources()
	if t.Order == ordering.Desc {
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
