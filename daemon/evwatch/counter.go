// Package evwatch runs the event sources of the daemon watchers on demand.
//
// A watcher like drbdmon reads the events a system tool prints, as
// "drbdsetup events2", for the instances holding a resource of its kind to
// evaluate their status again. Such a source is a process, and a connection
// to a service: a node holding no resource of its kind has no event to wait
// for, and a "virsh event" kept running would keep a socket-activated
// libvirtd running for nothing.
//
// The Counter counts the local resources of a kind from the instance
// statuses, the Runner runs the source while the count is not zero, and
// starts it again when it ends, the Debouncer publishes a change once the
// events about it settle, and StreamLines runs a tool printing an event per
// line, with a bounded wait once it is stopped.
package evwatch

import (
	"slices"
	"sort"
	"sync"

	"github.com/opensvc/om3/v3/core/instance"
)

// Counter counts the local resources a watcher has events for, by the key
// their events name them with, as the name of a libvirt domain or of a drbd
// resource, for each instance holding some.
type Counter struct {
	mu     sync.Mutex
	byPath map[string][]string
}

// NewCounter returns an empty Counter.
func NewCounter() *Counter {
	return &Counter{byPath: make(map[string][]string)}
}

// Set says the instance at path holds the resources of keys, and returns how
// many resources were counted before and after. An instance holding none is
// unset.
func (t *Counter) Set(path string, keys []string) (before, after int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	before = t.len()
	if len(keys) == 0 {
		delete(t.byPath, path)
	} else {
		t.byPath[path] = slices.Clone(keys)
	}
	return before, t.len()
}

// Unset says the instance at path holds no resource any more, as when its
// status is deleted.
func (t *Counter) Unset(path string) (before, after int) {
	return t.Set(path, nil)
}

// Len returns how many resources are counted.
func (t *Counter) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.len()
}

func (t *Counter) len() int {
	n := 0
	for _, keys := range t.byPath {
		n += len(keys)
	}
	return n
}

// Has says a counted resource is named key.
func (t *Counter) Has(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, keys := range t.byPath {
		if slices.Contains(keys, key) {
			return true
		}
	}
	return false
}

// Keys returns the names of the counted resources, each once, sorted, but for
// the resources counted with no name.
func (t *Counter) Keys() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	seen := make(map[string]bool)
	l := make([]string, 0)
	for _, keys := range t.byPath {
		for _, key := range keys {
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			l = append(l, key)
		}
	}
	sort.Strings(l)
	return l
}

// KeysOf returns the keys of the resources of the status whose driver type
// is one of types: the value of their status info infoKey, as the name of a
// libvirt domain. A resource of the type with no such value is counted all
// the same, with an empty key: it is one the watcher has events for, which
// can not be told apart.
func KeysOf(st instance.Status, infoKey string, types ...string) []string {
	l := make([]string, 0)
	for _, rs := range st.Resources {
		if !slices.Contains(types, rs.Type) {
			continue
		}
		key, _ := rs.Info[infoKey].(string)
		l = append(l, key)
	}
	sort.Strings(l)
	return l
}
