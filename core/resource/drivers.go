package resource

type (
	Drivers []Driver
)

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
