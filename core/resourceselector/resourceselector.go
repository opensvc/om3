package resourceselector

import (
	"strings"

	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		RID    string
		Tag    string
		Subset string

		lister ResourceLister
	}

	ResourceLister interface {
		Resources() resource.Drivers
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

func New(l ResourceLister, opts ...funcopt.O) *T {
	t := &T{
		lister: l,
	}
	_ = funcopt.Apply(t, opts...)
	return t
}

func (t T) Resources() resource.Drivers {
	if t.RID == "" && t.Tag == "" && t.Subset == "" {
		return t.lister.Resources()
	}
	m := make(map[string]resource.Driver)
	f := func(c rune) bool { return c == ',' }
	rids := strings.FieldsFunc(t.RID, f)
	tags := strings.FieldsFunc(t.Tag, f)
	subsets := strings.FieldsFunc(t.Subset, f)
	for _, r := range t.lister.Resources() {
		if _, ok := m[r.RID()]; ok {
			continue
		}
		for _, e := range rids {
			if r.MatchRID(e) {
				m[r.RID()] = r
			}
		}
		for _, e := range subsets {
			if r.MatchSubset(e) {
				m[r.RID()] = r
			}
		}
		for _, e := range tags {
			if r.MatchTag(e) {
				m[r.RID()] = r
			}
		}
	}
	l := make(resource.Drivers, len(m))
	i := 0
	for _, r := range m {
		l[i] = r
		i++
	}
	return l
}
