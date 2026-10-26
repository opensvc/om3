// Package uxid resolves the uid of a local process to its login and groups,
// as the name service of the node says them, without letting a name service
// not answering hold its callers.
//
// A lookup asks the name service, which may be a directory server not
// answering: nothing interrupts it. So a lookup is given up after a
// deadline, the lookup itself left to end on its own; at most a few run at
// once, a caller finding them all taken refused at once rather than piling
// threads up; and the answers are cached, the failures a short time, so a
// directory down only hits the identities not seen lately.
package uxid

import (
	"context"
	"errors"
	"fmt"
	"os/user"
	"strconv"
	"sync"
	"time"
)

type (
	// Identity is a uid as the name service says it.
	Identity struct {
		UID   uint32
		Login string

		// Groups are the names of the groups the login is a member of,
		// its primary group included.
		Groups []string
	}

	// LookupFunc resolves a uid, asking the name service.
	LookupFunc func(uid uint32) (Identity, error)

	// Resolver resolves uids with a deadline, a cap on the lookups
	// running, and a cache.
	Resolver struct {
		lookup      LookupFunc
		timeout     time.Duration
		positiveTTL time.Duration
		negativeTTL time.Duration
		slots       chan struct{}

		mu       sync.Mutex
		cache    map[uint32]entry
		inflight map[uint32]*call
	}

	entry struct {
		id      Identity
		err     error
		expires time.Time
	}

	call struct {
		done chan struct{}
		id   Identity
		err  error
	}
)

var (
	// ErrBusy is returned when the lookups running are at their cap: the
	// name service is slow, or not answering.
	ErrBusy = errors.New("the name service lookups running are at their cap")

	// ErrTimeout is returned when the name service did not answer in time.
	ErrTimeout = errors.New("the name service did not answer in time")
)

const (
	DefaultTimeout     = 3 * time.Second
	DefaultMaxInFlight = 8
	DefaultPositiveTTL = 5 * time.Minute
	DefaultNegativeTTL = 10 * time.Second
)

// New returns a resolver asking the name service through lookup, nil for
// the one of the system, with the default deadline, cap and cache times.
func New(lookup LookupFunc) *Resolver {
	if lookup == nil {
		lookup = SystemLookup
	}
	return &Resolver{
		lookup:      lookup,
		timeout:     DefaultTimeout,
		positiveTTL: DefaultPositiveTTL,
		negativeTTL: DefaultNegativeTTL,
		slots:       make(chan struct{}, DefaultMaxInFlight),
		cache:       make(map[uint32]entry),
		inflight:    make(map[uint32]*call),
	}
}

// Resolve returns the identity of uid, from the cache when it holds a fresh
// one. Concurrent callers of one uid share a lookup.
func (t *Resolver) Resolve(ctx context.Context, uid uint32) (Identity, error) {
	t.mu.Lock()
	if e, ok := t.cache[uid]; ok && time.Now().Before(e.expires) {
		t.mu.Unlock()
		return e.id, e.err
	}
	c, ok := t.inflight[uid]
	if !ok {
		select {
		case t.slots <- struct{}{}:
		default:
			t.mu.Unlock()
			return Identity{}, ErrBusy
		}
		c = &call{done: make(chan struct{})}
		t.inflight[uid] = c
		go t.run(uid, c)
	}
	t.mu.Unlock()

	timer := time.NewTimer(t.timeout)
	defer timer.Stop()
	select {
	case <-c.done:
		return c.id, c.err
	case <-timer.C:
		return Identity{}, fmt.Errorf("uid %d: %w", uid, ErrTimeout)
	case <-ctx.Done():
		return Identity{}, ctx.Err()
	}
}

// run looks uid up, holding a slot until the name service answers, however
// late: the slots bound the threads the lookups hold.
func (t *Resolver) run(uid uint32, c *call) {
	id, err := t.lookup(uid)
	ttl := t.positiveTTL
	if err != nil {
		ttl = t.negativeTTL
	}
	t.mu.Lock()
	c.id, c.err = id, err
	t.cache[uid] = entry{id: id, err: err, expires: time.Now().Add(ttl)}
	delete(t.inflight, uid)
	t.mu.Unlock()
	<-t.slots
	close(c.done)
}

// Flush forgets the identities cached.
func (t *Resolver) Flush() {
	t.mu.Lock()
	t.cache = make(map[uint32]entry)
	t.mu.Unlock()
}

// SystemLookup resolves uid with the name service of the system.
//
// Built without cgo, the os/user package reads /etc/passwd and /etc/group
// alone, and the accounts of a directory do not resolve.
func SystemLookup(uid uint32) (Identity, error) {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return Identity{}, err
	}
	gids, err := u.GroupIds()
	if err != nil {
		return Identity{}, err
	}
	id := Identity{UID: uid, Login: u.Username, Groups: make([]string, 0, len(gids))}
	for _, gid := range gids {
		g, err := user.LookupGroupId(gid)
		if err != nil {
			continue
		}
		id.Groups = append(id.Groups, g.Name)
	}
	return id, nil
}
