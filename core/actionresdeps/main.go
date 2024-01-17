package actionresdeps

import (
	"fmt"
	"strings"
	"sync"

	"github.com/opensvc/om3/util/xmap"
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

		// bMap holds the dependency relations, where A is the key and B the value.
		m map[depKey]bMap

		// actionMap is a action identity map. For example actionMap{"provision": "start"} tells the Store to consider the "provision" action as a "start".
		actionMap map[string]string
	}

	bMap map[string]interface{}
	kind int
)

const (
	// KindSelect selects the dependency target <B> upon action <Action> so <Action> is also executed on <B> without changing <B> position in the action sequence.
	KindSelect kind = iota

	// KindAct selects the dependency target <B> upon action <Action> so <Action> is executed on <B> before <A>.
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

func (t Dep) key() depKey {
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
	t.actionMap = make(map[string]string)
	return &t
}

func (t *Store) String() string {
	s := ""
	for key, bs := range t.m {
		s += fmt.Sprintf("on %s %s, %s depends on %s\n", key.Action, key.Kind, key.A, strings.Join(xmap.Keys(bs), ","))
	}
	return s
}

func (t *Store) SetActionMap(m map[string]string) {
	t.Lock()
	defer t.Unlock()
	t.actionMap = m
}

func (t *Store) RegisterSlice(deps []Dep) {
	for _, dep := range deps {
		t.Register(dep)
	}
}

func (t *Store) Register(dep Dep) {
	t.Lock()
	defer t.Unlock()
	key := dep.key()
	bs, ok := t.m[key]
	if !ok {
		t.m[key] = make(bMap)
		bs, _ = t.m[key]
	}
	bs[dep.B] = nil
}

func (t *Store) SelectDependencies(action, rid string) []string {
	return t.dependencies(action, rid, KindSelect)
}

func (t *Store) ActDependencies(action, rid string) []string {
	return t.dependencies(action, rid, KindAct)
}

func (t *Store) mappedAction(action string) string {
	if a, ok := t.actionMap[action]; ok {
		return a
	}
	return action
}

func (t *Store) dependencies(action, rid string, kd kind) []string {
	action = t.mappedAction(action)
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
