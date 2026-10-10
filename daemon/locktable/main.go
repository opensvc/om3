// Package locktable holds the cluster locks: the table the node speaking for
// the cluster grants them from, and the record every node keeps of the locks
// its own clients hold.
//
// A lock is granted for a lease, and lapses when the lease ends whatever its
// holder became: a process that died holding one does not hold it forever.
//
// A lease is counted on the clock of the node keeping the lock, which is the
// monotonic clock of the process: a step of the wall clock does not end it
// early. The nodes pass a lease to one another as the time left, which each
// adds to its own clock, so their clocks need not agree on the time, only
// tick at the same rate. The holder is to be done before the lease ends,
// which it counts on its own clock the same way.
//
// The table is in memory, and the node speaking for the cluster changes. A
// node taking the speaking over rebuilds its table from what every node says
// its clients hold, and what it granted while it spoke, before it grants
// anything: its own table is the one it had when it last spoke, or none. The
// grants are read too because a lock granted to a request a node handed over
// is held before that node hears of it.
package locktable

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type (
	// Lock is a cluster lock granted.
	Lock struct {
		Name string `json:"name"`
		ID   string `json:"id"`

		// Node is the node whose client holds the lock.
		Node string `json:"node"`

		// Holder says who holds it, for the reader of a listing.
		Holder string `json:"holder,omitempty"`

		AcquiredAt time.Time `json:"acquired_at"`
		ExpiresAt  time.Time `json:"expires_at"`
	}

	// Request is a lock asked for.
	Request struct {
		Name   string
		Node   string
		Holder string

		// Lease is how long the lock is held once granted, unless released
		// before.
		Lease time.Duration

		// Wait is how long the request waits for a lock held by another.
		Wait time.Duration
	}

	// Table is the locks the node speaking for the cluster granted.
	Table struct {
		mu    sync.Mutex
		locks map[string]Lock

		// rebuilt says the table holds every lock the cluster holds. It is
		// false until the table was rebuilt from the nodes, and again when
		// the node stops or starts speaking for the cluster.
		rebuilt bool

		// generation counts the drops, so a rebuild that started before one
		// does not mark rebuilt a table it read the nodes for too early.
		generation uint64

		// former is the locks the table held when it was last dropped, kept
		// until their leases end. A lock granted to a request handed over
		// is held from the moment it is granted, and recorded by the node
		// of its client only once the answer is back there: a table rebuilt
		// in between finds it here, on the node that granted it.
		former map[string]Lock

		// released is the ids of the locks this node was told were released,
		// until the longest lease any of them could have ends. A rebuild
		// reads the nodes while the word of a release is still on its way to
		// some of them, and one that has not heard of it yet still reports
		// the lock: skipping the ids released keeps it from coming back.
		released map[string]time.Time

		// changed is closed, and replaced, whenever a lock is released or
		// the table is dropped, which wakes the requests waiting.
		changed chan struct{}

		now func() time.Time
	}

	// Held is the locks the clients of a node hold, which is what a node
	// taking the speaking over rebuilds its table from.
	Held struct {
		mu    sync.Mutex
		locks map[string]Lock
		now   func() time.Time
	}

	// ErrHeld is the error of a lock still held by another at the end of the
	// wait.
	ErrHeld struct {
		Lock Lock
	}
)

const (
	// DefaultLease is the lease of a request naming none.
	DefaultLease = 30 * time.Second

	// MaxLease is the longest lease granted. A lock is for a short critical
	// section, and the lease is what frees it when its holder dies.
	MaxLease = 10 * time.Minute
)

var (
	// ErrNotRebuilt is the error of a request to a table not rebuilt yet.
	ErrNotRebuilt = errors.New("the cluster lock table is not rebuilt from the nodes yet")

	// SpeakerTable is the table this node grants from while it speaks for the
	// cluster.
	SpeakerTable = NewTable()

	// LocalHeld is the locks the clients of this node hold.
	LocalHeld = NewHeld()
)

func (t ErrHeld) Error() string {
	return fmt.Sprintf("lock %s is held by %s on %s until %s", t.Lock.Name, t.Lock.Holder, t.Lock.Node, t.Lock.ExpiresAt.Format(time.RFC3339))
}

// NewTable returns an empty table, not rebuilt.
func NewTable() *Table {
	return &Table{
		locks:    make(map[string]Lock),
		former:   make(map[string]Lock),
		released: make(map[string]time.Time),
		changed:  make(chan struct{}),
		now:      time.Now,
	}
}

// NewHeld returns an empty record of held locks.
func NewHeld() *Held {
	return &Held{
		locks: make(map[string]Lock),
		now:   time.Now,
	}
}

// Lease returns the lease a request is granted: the default when it names
// none, and the longest granted when it names more.
func Lease(d time.Duration) time.Duration {
	switch {
	case d <= 0:
		return DefaultLease
	case d > MaxLease:
		return MaxLease
	default:
		return d
	}
}

// IsRebuilt says the table holds every lock the cluster holds.
func (t *Table) IsRebuilt() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.rebuilt
}

// Generation returns the generation a rebuild is for: the nodes are read
// after it is, and the table is rebuilt from them only if it was not dropped
// meanwhile.
func (t *Table) Generation() uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.generation
}

// Rebuild replaces the locks of the table with the ones the nodes say their
// clients hold, and marks it rebuilt. It says false, and changes nothing, when
// the table was dropped since generation, which is the nodes being read for a
// table this node may no longer keep.
//
// Two nodes saying they hold the same lock is a lock two speakers granted
// apart, which only the end of one of the leases settles: the one held the
// longest is kept, and the other is still held by its client, which is
// returned.
func (t *Table) Rebuild(generation uint64, locks []Lock) ([]Lock, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if generation != t.generation {
		return nil, false
	}
	now := t.now()
	t.locks = make(map[string]Lock, len(locks))
	conflicts := make([]Lock, 0)
	t.pruneReleased(now)
	for _, lock := range locks {
		if !lock.ExpiresAt.After(now) {
			continue
		}
		if _, ok := t.released[lock.ID]; ok {
			continue
		}
		if kept, ok := t.locks[lock.Name]; ok && kept.ID != lock.ID {
			if lock.ExpiresAt.After(kept.ExpiresAt) {
				t.locks[lock.Name], lock = lock, kept
			}
			conflicts = append(conflicts, lock)
			continue
		}
		t.locks[lock.Name] = lock
	}
	t.rebuilt = true
	return conflicts, true
}

// Drop forgets the table, which is no longer this node's to keep, or is about
// to be again. The requests waiting are answered ErrNotRebuilt.
func (t *Table) Drop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for name, lock := range t.locks {
		t.former[name] = lock
	}
	t.locks = make(map[string]Lock)
	t.rebuilt = false
	t.generation++
	t.wake()
}

// wake answers the requests waiting. The caller holds t.mu.
func (t *Table) wake() {
	close(t.changed)
	t.changed = make(chan struct{})
}

// Acquire grants the lock req names, waiting for as long as req says when
// another holds it, and answers ErrHeld when the wait ends with the lock still
// held.
func (t *Table) Acquire(ctx context.Context, req Request) (Lock, error) {
	if req.Name == "" {
		return Lock{}, errors.New("a lock needs a name")
	}
	deadline := t.now().Add(req.Wait)
	for {
		t.mu.Lock()
		if !t.rebuilt {
			t.mu.Unlock()
			return Lock{}, ErrNotRebuilt
		}
		now := t.now()
		held, ok := t.locks[req.Name]
		if !ok || !held.ExpiresAt.After(now) {
			lock := Lock{
				Name:       req.Name,
				ID:         uuid.NewString(),
				Node:       req.Node,
				Holder:     req.Holder,
				AcquiredAt: now,
				ExpiresAt:  now.Add(Lease(req.Lease)),
			}
			t.locks[req.Name] = lock
			t.mu.Unlock()
			return lock, nil
		}
		changed := t.changed
		t.mu.Unlock()

		remaining := deadline.Sub(now)
		if remaining <= 0 {
			return Lock{}, ErrHeld{Lock: held}
		}
		// The lease ending frees the lock as a release does, and nothing
		// says so but the time.
		timeout := held.ExpiresAt.Sub(now)
		if remaining < timeout {
			timeout = remaining
		}
		timer := time.NewTimer(timeout)
		select {
		case <-ctx.Done():
			timer.Stop()
			return Lock{}, ctx.Err()
		case <-changed:
		case <-timer.C:
		}
		timer.Stop()
	}
}

// Release frees the lock name granted under id, and returns it, with whether
// it was held under it. A lock that lapsed, or was granted again since, is not
// this id's to release.
func (t *Table) Release(name, id string) (Lock, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	held, ok := t.locks[name]
	if former, ok := t.former[name]; ok && former.ID == id {
		delete(t.former, name)
	}
	if !ok || held.ID != id {
		return Lock{}, false
	}
	t.released[id] = t.now().Add(MaxLease)
	delete(t.locks, name)
	t.wake()
	return held, true
}

// List returns the locks held, by name.
func (t *Table) List() []Lock {
	t.mu.Lock()
	defer t.mu.Unlock()
	return list(t.locks, t.now())
}

// Granted returns the locks this node granted and whose lease has not ended:
// the ones of its table, and the ones it held when it was last dropped. It is
// what a node rebuilding its table reads of this one, besides the locks its
// clients hold.
func (t *Table) Granted() []Lock {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	l := list(t.former, now)
	for _, lock := range list(t.locks, now) {
		if former, ok := t.former[lock.Name]; ok && former.ID == lock.ID {
			continue
		}
		l = append(l, lock)
	}
	return l
}

// Forget says a lock was released: it is dropped from the locks this node
// granted before its table was dropped, from its table, where a rebuild may
// have brought it back from a node that had not heard of the release yet,
// and its id is kept from the rebuilds to come.
func (t *Table) Forget(name, id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	t.pruneReleased(now)
	t.released[id] = now.Add(MaxLease)
	if lock, ok := t.former[name]; ok && lock.ID == id {
		delete(t.former, name)
	}
	if lock, ok := t.locks[name]; ok && lock.ID == id {
		delete(t.locks, name)
		t.wake()
	}
}

// pruneReleased forgets the ids released whose lease has ended for sure. The
// caller holds t.mu.
func (t *Table) pruneReleased(now time.Time) {
	for id, until := range t.released {
		if !until.After(now) {
			delete(t.released, id)
		}
	}
}

// Add records a lock a client of this node was granted.
func (t *Held) Add(lock Lock) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.locks[lock.Name] = lock
}

// Remove forgets the lock name held under id.
func (t *Held) Remove(name, id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if held, ok := t.locks[name]; ok && held.ID == id {
		delete(t.locks, name)
	}
}

// List returns the locks the clients of this node hold, by name.
func (t *Held) List() []Lock {
	t.mu.Lock()
	defer t.mu.Unlock()
	return list(t.locks, t.now())
}

// list returns the locks of m whose lease has not ended, sorted by name, and
// forgets the others.
func list(m map[string]Lock, now time.Time) []Lock {
	l := make([]Lock, 0, len(m))
	for name, lock := range m {
		if !lock.ExpiresAt.After(now) {
			delete(m, name)
			continue
		}
		l = append(l, lock)
	}
	sort.Slice(l, func(i, j int) bool { return l[i].Name < l[j].Name })
	return l
}
