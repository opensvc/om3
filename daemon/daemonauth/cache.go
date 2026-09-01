package daemonauth

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// cache holds the result of an authentication for a few seconds, keyed by
// the credential that produced it.
//
// It is what keeps a client polling the api from re-reading and
// decrypting a usr object, or verifying the same rsa signature, on every
// request. The entries are the answer to a credential, not to a user: a
// second password for the same username is a different key, and misses.
//
// The key is a digest rather than the credential itself, so that a heap
// dump of the daemon does not hand over the passwords and tokens that
// were presented to it.
type cache struct {
	mu sync.Mutex

	// m is bounded because its keys come from the request. A client can
	// present a new token, and so create an entry, as fast as the rate
	// limiter lets it.
	m   map[string]cacheEntry
	max int
}

type cacheEntry struct {
	info     *Info
	expireAt time.Time
}

// cacheTTL is how long an authentication is remembered. It is short
// enough that a usr object edit, a grant change or a deleted user takes
// effect while the operator is still watching, and long enough to cover
// a burst of requests from one client.
const cacheTTL = 5 * time.Second

// cacheMaxEntries is the point where the cache stops being a cache and
// starts being a leak.
const cacheMaxEntries = 1000

func newCache() *cache {
	return &cache{m: make(map[string]cacheEntry), max: cacheMaxEntries}
}

// cacheKey returns the key an authentication is remembered under. The
// parts are joined with a separator that cannot appear in a digest, so
// that two different credentials cannot produce the same key.
func cacheKey(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (c *cache) get(key string) (*Info, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expireAt) {
		delete(c.m, key)
		return nil, false
	}
	return e.info, true
}

// set remembers info for ttl, or for cacheTTL when ttl is longer than
// that. A token valid for a day is still only cached for the few seconds
// a burst lasts.
func (c *cache) set(key string, info *Info, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	if ttl > cacheTTL {
		ttl = cacheTTL
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= c.max {
		c.evict()
	}
	c.m[key] = cacheEntry{info: info, expireAt: time.Now().Add(ttl)}
}

// evict drops what has expired, and everything if that was not enough.
// Dropping a live entry costs one authentication, so there is nothing to
// pick carefully here.
func (c *cache) evict() {
	now := time.Now()
	for k, e := range c.m {
		if now.After(e.expireAt) {
			delete(c.m, k)
		}
	}
	if len(c.m) >= c.max {
		clear(c.m)
	}
}
