package volsignal

import (
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

type (
	routeMap map[syscall.Signal]map[string]any

	T struct {
		data routeMap
	}

	// Route is a relation between a signal number and the id of a resource supporting signaling
	Route struct {
		Signum syscall.Signal
		RID    string
	}
)

func New(expressions ...string) *T {
	t := &T{
		data: make(routeMap),
	}
	for _, expression := range expressions {
		t.Parse(expression)
	}
	return t
}

func (t *T) Merge(other *T) {
	if other == nil {
		return
	}
	for sigNum, rids := range other.data {
		if _, ok := t.data[sigNum]; !ok {
			t.data[sigNum] = make(map[string]interface{})
		}
		for rid := range rids {
			t.data[sigNum][rid] = nil
		}
	}
}

func (t *T) Parse(s string) {
	for _, e := range strings.Fields(s) {
		l := strings.SplitN(e, ":", 2)
		if len(l) != 2 {
			continue
		}
		sigName := strings.ToUpper(l[0])
		if !strings.HasPrefix(sigName, "SIG") {
			sigName = "SIG" + sigName
		}
		sigNum := unix.SignalNum(sigName)
		if sigNum == 0 {
			continue
		}
		if _, ok := t.data[sigNum]; !ok {
			t.data[sigNum] = make(map[string]interface{})
		}
		for _, rid := range strings.Split(l[1], ",") {
			t.data[sigNum][rid] = nil
		}
	}
}

func (t *T) Routes() []Route {
	routes := make([]Route, 0)
	for i, ridmap := range t.data {
		for rid := range ridmap {
			routes = append(routes, Route{
				Signum: i,
				RID:    rid,
			})
		}
	}
	return routes
}
