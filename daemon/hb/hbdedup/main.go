// Package hbdedup drops the decoding of a heartbeat frame a peer already
// delivered over another heartbeat link.
//
// The sender encrypts a heartbeat message once and hands the very same bytes
// to every configured tx driver, so a cluster running n heartbeats delivers n
// byte identical frames per message. Every rx driver used to decrypt,
// decompress and unmarshal its copy in full, and hb.msgFromRx then dropped
// all but the first on their UpdatedAt. The work was done n times to be kept
// once, and it is the most expensive work the idle daemon does.
//
// A Cache holds the frames already decoded and delivered, so the drivers can
// skip the decoding of the copies instead of the daemon discarding their
// result.
package hbdedup

import (
	"context"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
)

type (
	// Key identifies a frame. It is a hash, so two distinct frames sharing
	// one are dropped as duplicates. Distinct frames have to collide on
	// both the xxhash and the length for that, and the loss costs the next
	// heartbeat of the peer, a few seconds later, carrying the state again.
	Key struct {
		hash uint64
		size int
	}

	// Cache remembers the frames delivered during the last two windows.
	//
	// Expiry is generational rather than per entry: the current generation
	// becomes the previous one and a new current one is started once a
	// window has passed, which bounds the memory to the frames of two
	// windows without walking the entries.
	Cache struct {
		mu sync.Mutex

		current  map[Key]string
		previous map[Key]string

		window    time.Duration
		rotatedAt time.Time
	}

	contextKey int
)

const (
	cacheKey contextKey = 0

	// DefaultWindow is how long a frame is remembered, at least. It has to
	// cover the spread between the first and the last link delivering a
	// message, which is a network delay, not a heartbeat interval.
	DefaultWindow = 30 * time.Second
)

// NewCache returns a Cache remembering a frame for at least window.
func NewCache(window time.Duration) *Cache {
	if window <= 0 {
		window = DefaultWindow
	}
	return &Cache{
		current:   make(map[Key]string),
		window:    window,
		rotatedAt: time.Now(),
	}
}

// NewKey returns the Key of frame.
func NewKey(frame []byte) Key {
	return Key{hash: xxhash.Sum64(frame), size: len(frame)}
}

// Seen returns the nodename the frame was delivered for, and true, when
// another link already delivered it.
//
// A caller getting true still has to report its link alive: a link delivering
// nothing but duplicates is a working link, and dropping its liveness report
// would have it declared stale.
func (t *Cache) Seen(k Key) (string, bool) {
	if t == nil {
		return "", false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if nodename, ok := t.current[k]; ok {
		return nodename, true
	}
	nodename, ok := t.previous[k]
	return nodename, ok
}

// Delivered records a frame as decoded and handed over. nodename is the
// Nodename of the message it carried, which is the sender: the receivers
// checking a frame came from the node they expect, like the disk one
// verifying no peer wrote in another's slot, compare against it.
//
// It must be called after the message reached the daemon, never before: a
// frame recorded by a link that then fails to deliver it would be skipped by
// the links still to come, and the message lost. Recording it late only costs
// the decoding of a copy arriving in the meantime.
func (t *Cache) Delivered(k Key, nodename string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if now := time.Now(); now.Sub(t.rotatedAt) >= t.window {
		t.previous = t.current
		t.current = make(map[Key]string, len(t.previous))
		t.rotatedAt = now
	}
	t.current[k] = nodename
}

// Len returns the number of frames remembered, for the tests.
func (t *Cache) Len() int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.current) + len(t.previous)
}

func ContextWithCache(ctx context.Context, c *Cache) context.Context {
	return context.WithValue(ctx, cacheKey, c)
}

// CacheFromContext returns the context Cache, or nil when the context carries
// none. A nil *Cache is usable: it remembers nothing, so every frame is
// decoded, which is the behaviour that predates this package.
func CacheFromContext(ctx context.Context) *Cache {
	c, _ := ctx.Value(cacheKey).(*Cache)
	return c
}
