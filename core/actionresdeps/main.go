package actionresdeps

import (
	"fmt"
	"strings"
	"sync"

	"opensvc.com/opensvc/util/xmap"
)

type (
	// Dep is a {action, depending rid, depended on} rid single relation.
	Dep struct {
		Action string
		Kind   kind
		A      string
		B      string
	}

	depKey struct {
		Action string
		Kind   kind
		A      string
	}

	// Store is the action resource dependencies data store.
	Store struct {
		sync.Mutex
		m map[depKey]bMap
	}

	bMap map[string]interface{}
	kind int
)

const (
	KindSelect kind = iota
	KindAct
)

func (t kind) String() string {
	switch t {
	case KindSelect:
		return "select"
	case KindAct:
		return "act"
	default:
		return fmt.Sprintf("unknown (%d)", t)
	}
}

func (t Dep) Key() depKey {
	o := depKey{
		Action: t.Action,
		Kind:   t.Kind,
		A:      t.A,
	}
	return o
}

func NewStore() *Store {
	t := Store{}
	t.m = make(map[depKey]bMap)
	return &t
}

func (t Store) String() string {
	s := ""
	for key, bs := range t.m {
		s += fmt.Sprintf("on %s %s, %s depends on %s\n", key.Action, key.Kind, key.A, strings.Join(xmap.Keys(bs), ","))
	}
	return s
}

func (t *Store) RegisterSlice(deps []Dep) {
	for _, dep := range deps {
		t.Register(dep)
	}
}

func (t *Store) Register(dep Dep) {
	t.Lock()
	defer t.Unlock()
	key := dep.Key()
	bs, ok := t.m[key]
	if !ok {
		t.m[key] = make(bMap)
		bs, _ = t.m[key]
	}
	bs[dep.B] = nil
}

func (t Store) SelectDependencies(action, rid string) []string {
	return t.dependencies(action, rid, KindSelect)
}

func (t Store) ActDependencies(action, rid string) []string {
	return t.dependencies(action, rid, KindAct)
}

func (t Store) dependencies(action, rid string, kd kind) []string {
	switch action {
	case "provision", "start":
		action = "start"
	case "shutdown", "unprovision", "stop", "toc":
		action = "stop"
	default:
		return []string{}
	}
	key := depKey{
		Action: action,
		Kind:   kd,
		A:      rid,
	}
	bs, ok := t.m[key]
	if !ok {
		return []string{}
	}
	return xmap.Keys(bs)
}
