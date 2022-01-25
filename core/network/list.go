package network

import (
	"sort"
)

func List(noder Noder) []string {
	l := make([]string, 0)
	for _, n := range Networks(noder) {
		l = append(l, n.Name())
	}
	sort.Strings(l)
	return l
}
