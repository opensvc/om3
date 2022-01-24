package network

import (
	"sort"

	"opensvc.com/opensvc/core/object"
)

func List(n *object.Node) []string {
	l := make([]string, 0)
	for _, n := range Networks(n) {
		l = append(l, n.Name())
	}
	sort.Strings(l)
	return l
}
