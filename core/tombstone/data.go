package tombstone

import (
	"slices"
	"sync"
)

type (
	// DataHolder holds the records every node publishes, by node and id.
	DataHolder struct {
		sync.RWMutex
		data map[string]map[string]Record
	}
)

// Data is the package holder of the records the nodes publish.
var Data *DataHolder

// InitData resets the package holder.
func InitData() {
	Data = NewData()
}

func init() {
	InitData()
}

func NewData() *DataHolder {
	return &DataHolder{data: make(map[string]map[string]Record)}
}

// Set holds the record node publishes.
func (t *DataHolder) Set(node string, rec Record) {
	t.Lock()
	defer t.Unlock()
	m, ok := t.data[node]
	if !ok {
		m = make(map[string]Record)
		t.data[node] = m
	}
	m[rec.OrchestrationID] = rec.DeepCopy()
}

// Unset forgets the record of id node published.
func (t *DataHolder) Unset(node, id string) {
	t.Lock()
	defer t.Unlock()
	if m, ok := t.data[node]; ok {
		delete(m, id)
		if len(m) == 0 {
			delete(t.data, node)
		}
	}
}

// GetByNode returns a copy of the records node publishes, by id.
func (t *DataHolder) GetByNode(node string) map[string]Record {
	t.RLock()
	defer t.RUnlock()
	m := make(map[string]Record, len(t.data[node]))
	for id, rec := range t.data[node] {
		m[id] = rec.DeepCopy()
	}
	return m
}

// Get returns a copy of the record of id node publishes.
func (t *DataHolder) Get(node, id string) (Record, bool) {
	t.RLock()
	defer t.RUnlock()
	rec, ok := t.data[node][id]
	if !ok {
		return Record{}, false
	}
	return rec.DeepCopy(), true
}

// Publishers returns the sorted nodes publishing the record of id.
func (t *DataHolder) Publishers(id string) []string {
	t.RLock()
	defer t.RUnlock()
	l := make([]string, 0)
	for node, m := range t.data {
		if _, ok := m[id]; ok {
			l = append(l, node)
		}
	}
	slices.Sort(l)
	return l
}

// Merged returns the records of every node merged per id.
func (t *DataHolder) Merged() map[string]Record {
	t.RLock()
	defer t.RUnlock()
	merged := make(map[string]Record)
	for _, m := range t.data {
		for id, rec := range m {
			if cur, ok := merged[id]; ok {
				merged[id], _ = Merge(cur, rec)
			} else {
				merged[id] = rec.DeepCopy()
			}
		}
	}
	return merged
}

// MergedOf returns the records of path of every node merged per id, copying
// none of the others: what the discovery asks at each fetch decision.
func (t *DataHolder) MergedOf(path string) map[string]Record {
	t.RLock()
	defer t.RUnlock()
	merged := make(map[string]Record)
	for _, m := range t.data {
		for id, rec := range m {
			if rec.Path != path {
				continue
			}
			if cur, ok := merged[id]; ok {
				merged[id], _ = Merge(cur, rec)
			} else {
				merged[id] = rec.DeepCopy()
			}
		}
	}
	return merged
}

// PendingHolders returns the sorted nodes pending in a record of path, of
// mode, or of any mode for an empty mode, the copies of every node merged.
func (t *DataHolder) PendingHolders(path string, mode HolderMode) []string {
	set := make(map[string]bool)
	for _, rec := range t.MergedOf(path) {
		for _, n := range rec.PendingHolders(mode) {
			set[n] = true
		}
	}
	l := make([]string, 0, len(set))
	for n := range set {
		l = append(l, n)
	}
	slices.Sort(l)
	return l
}

// IsPending says whether node is pending in a record of path, the copies of
// every node merged.
func (t *DataHolder) IsPending(path, node string) bool {
	return slices.Contains(t.PendingHolders(path, ""), node)
}
