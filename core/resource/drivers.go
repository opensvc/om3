package resource

type (
	Drivers []Driver
)

func (t Drivers) Len() int      { return len(t) }
func (t Drivers) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t Drivers) Less(i, j int) bool {
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
		return id1.Name < id2.Name
	}
}

//
// Has returns true if t has a driver whose RID() is the same
// as d.
//
func (t Drivers) Has(d Driver) bool {
	rid := d.RID()
	for _, r := range t {
		if r.RID() == rid {
			return true
		}
	}
	return false
}

//
// Intersection returns a list of drivers ordered like t and
// purged from drivers in other.
//
func (t Drivers) Intersection(other Drivers) Drivers {
	l := make(Drivers, 0)
	for _, r := range t {
		if other.Has(r) {
			l = append(l, r)
		}
	}
	return l
}

//
// Union return a deduplicated list containing all drivers from
// t and other.
//
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
