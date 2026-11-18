package tombstone

import (
	"slices"
	"sync"
	"time"
)

type (
	// DataHolder holds the records every node publishes, by node and id.
	DataHolder struct {
		sync.RWMutex
		data map[string]map[string]entry
	}

	entry struct {
		rec     Record
		trusted bool
	}

	// Killer is a record killing a config, and whether it is trusted:
	// heard live from its node, the local node up, rather than read from
	// a full data, as at a restart or a reconnection.
	Killer struct {
		Node    string
		Record  Record
		Trusted bool
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
	return &DataHolder{data: make(map[string]map[string]entry)}
}

// Set holds the record node publishes, trusted or not. A record held keeps
// the trust it was first held with.
func (t *DataHolder) Set(node string, rec Record, trusted bool) {
	t.Lock()
	defer t.Unlock()
	m, ok := t.data[node]
	if !ok {
		m = make(map[string]entry)
		t.data[node] = m
	}
	if cur, ok := m[rec.OrchestrationID]; ok {
		// The first hearing decides: a record is published live once,
		// as its delete writes it, so a record heard again, as one its
		// node republishes after a restart, is not trusted anew.
		trusted = cur.trusted
	}
	m[rec.OrchestrationID] = entry{rec: rec.DeepCopy(), trusted: trusted}
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

// Get returns the record of id node publishes.
func (t *DataHolder) Get(node, id string) (Record, bool) {
	t.RLock()
	defer t.RUnlock()
	e, ok := t.data[node][id]
	return e.rec.DeepCopy(), ok
}

// GetByNode returns the records node publishes, by id.
func (t *DataHolder) GetByNode(node string) map[string]Record {
	t.RLock()
	defer t.RUnlock()
	m := make(map[string]Record, len(t.data[node]))
	for id, e := range t.data[node] {
		m[id] = e.rec.DeepCopy()
	}
	return m
}

// All returns every record, by node and id.
func (t *DataHolder) All() map[string]map[string]Record {
	t.RLock()
	defer t.RUnlock()
	m := make(map[string]map[string]Record, len(t.data))
	for node, recs := range t.data {
		m[node] = make(map[string]Record, len(recs))
		for id, e := range recs {
			m[node][id] = e.rec.DeepCopy()
		}
	}
	return m
}

// KilledBy returns the records killing a config of path modified at
// mtime, sorted by node and id, a trusted one first.
func (t *DataHolder) KilledBy(path string, mtime time.Time) []Killer {
	t.RLock()
	defer t.RUnlock()
	var l []Killer
	for node, recs := range t.data {
		for _, e := range recs {
			if e.rec.Kills(path, mtime) {
				l = append(l, Killer{Node: node, Record: e.rec.DeepCopy(), Trusted: e.trusted})
			}
		}
	}
	slices.SortFunc(l, func(a, b Killer) int {
		switch {
		case a.Trusted != b.Trusted:
			if a.Trusted {
				return -1
			}
			return 1
		case a.Node != b.Node:
			if a.Node < b.Node {
				return -1
			}
			return 1
		case a.Record.OrchestrationID < b.Record.OrchestrationID:
			return -1
		case a.Record.OrchestrationID > b.Record.OrchestrationID:
			return 1
		}
		return 0
	})
	return l
}

// Kills says whether a record kills a config of path modified at mtime.
func (t *DataHolder) Kills(path string, mtime time.Time) bool {
	t.RLock()
	defer t.RUnlock()
	for _, recs := range t.data {
		for _, e := range recs {
			if e.rec.Kills(path, mtime) {
				return true
			}
		}
	}
	return false
}
