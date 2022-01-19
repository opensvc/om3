package network

import (
	"sort"
)

type (
	manager interface {
		Networks() []Networker
	}
	By           func(p1, p2 *Status) bool
	statusSorter struct {
		data []Status
		by   func(p1, p2 *Status) bool // Closure used in the Less method.
	}
)

func (by By) Sort(l []Status) {
	s := &statusSorter{
		data: l,
		by:   by, // The Sort method's receiver is the function (closure) that defines the sort order.
	}
	sort.Sort(s)
}

func (t statusSorter) Len() int {
	return len(t.data)
}

func (t statusSorter) Less(i, j int) bool {
	return t.by(&t.data[i], &t.data[j])
}

func (t statusSorter) Swap(i, j int) {
	t.data[i], t.data[j] = t.data[j], t.data[i]
}
