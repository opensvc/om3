package resource

import (
	"slices"
	"sort"
)

type (
	Drivers   []Driver
	sortKeyer interface {
		SortKey() string
	}
	LinkToer interface {
		LinkTo() string
	}
	LinkNameser interface {
		LinkNames() []string
	}
)

func (t Drivers) Len() int      { return len(t) }
func (t Drivers) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t Drivers) Less(i, j int) bool {
	sk := func(d Driver) string {
		switch i := d.(type) {
		case sortKeyer:
			return i.SortKey()
		default:
			return d.ID().Name
		}
	}

	id1 := t[i].ID()
	id2 := t[j].ID()
	switch {
	case id1.DriverGroup() < id2.DriverGroup():
		return true
	case id1.DriverGroup() > id2.DriverGroup():
		return false
		// same driver group
	case t[i].RSubset() < t[j].RSubset():
		return true
	case t[i].RSubset() > t[j].RSubset():
		return false
		// and same subset
	default:
		return sk(t[i]) < sk(t[j])
	}
}

// Has returns true if t has a driver whose RID() is the same
// as d.
func (t Drivers) Has(d Driver) bool {
	rid := d.RID()
	return t.HasRID(rid)
}

// HasRID returns true if t has a driver whose RID() is the same
// as rid.
func (t Drivers) HasRID(rid string) bool {
	for _, r := range t {
		if r.RID() == rid {
			return true
		}
	}
	return false
}

// ResolveLink returns the driver intstance targeted by <to>
func (t Drivers) ResolveLink(to string) (Driver, bool) {
	for _, r := range t {
		i, ok := r.(LinkNameser)
		if !ok {
			continue
		}
		names := i.LinkNames()
		if slices.Contains(names, to) {
			return r, true
		}
	}
	return nil, false
}

func (t Drivers) LinkersRID(names []string) []string {
	l := t.Linkers(names)
	rids := make([]string, len(l))
	for i, r := range l {
		rids[i] = r.RID()
	}
	return rids
}

func (t Drivers) Linkers(names []string) Drivers {
	l := make(Drivers, 0)
	for _, r := range t {
		i, ok := r.(LinkToer)
		if !ok {
			continue
		}
		to := i.LinkTo()
		if slices.Contains(names, to) {
			l = append(l, r)
		}
	}
	return l
}

// Intersection returns a list of drivers ordered like t and
// purged from drivers in other.
func (t Drivers) Intersection(other Drivers) Drivers {
	l := make(Drivers, 0)
	for _, r := range t {
		if other.Has(r) {
			l = append(l, r)
		}
	}
	return l
}

// Union return a deduplicated list containing all drivers from
// t and other.
func (t Drivers) Union(other Drivers) Drivers {
	l := make(Drivers, 0)
	l = append(l, t...)
	for _, r := range other {
		if !l.Has(r) {
			l = append(l, r)
		}
	}
	return l
}

func (t Drivers) GetRID(rid string) Driver {
	for _, r := range t {
		if r.RID() == rid {
			return r
		}
	}
	return nil
}

func (t Drivers) Add(r Driver) Drivers {
	if t.Has(r) {
		return t
	}
	return append(t, r)
}

// Sort sorts the driver list.
func (t Drivers) Sort() {
	sort.Sort(t)
}

// Reverse reverses the driver list sort.
func (t Drivers) Reverse() {
	sort.Sort(sort.Reverse(t))
}

// Truncate returns the drivers list from first to the driver with <rid>.
// If rid is not set, return the whole driver list.
// The second return value is true if the rid was found, whatever the
// truncation done.
func (t Drivers) Truncate(rid string) (Drivers, bool) {
	if rid == "" {
		return t, false
	}
	l := make(Drivers, 0)
	for _, r := range t {
		l = append(l, r)
		if r.RID() == rid {
			return l, true
		}
	}
	return l, false
}
