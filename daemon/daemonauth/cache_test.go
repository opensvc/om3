package daemonauth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheKeyDependsOnEveryPart(t *testing.T) {
	// The parts are joined before hashing, so a key built from the pieces
	// of one credential must not collide with a key built from the pieces
	// of another, however they are split.
	assert.NotEqual(t, cacheKey("user", "ab", "c"), cacheKey("user", "a", "bc"))
	assert.Equal(t, cacheKey("user", "a", "b"), cacheKey("user", "a", "b"))
}

func TestCacheKeyDoesNotHoldTheCredential(t *testing.T) {
	assert.NotContains(t, cacheKey(StrategyUser, "alice", "secret"), "secret")
}

func TestCacheForgetsWhatHasExpired(t *testing.T) {
	c := newCache()
	info := &Info{Username: "alice"}
	c.set("k", info, time.Millisecond)
	got, ok := c.get("k")
	require.True(t, ok)
	assert.Same(t, info, got)

	time.Sleep(2 * time.Millisecond)
	_, ok = c.get("k")
	assert.False(t, ok, "an entry must not be served after its ttl")
}

func TestCacheCapsTheTTL(t *testing.T) {
	// A token valid for a day must not be remembered for a day: a grant
	// revoked in the meantime has to take effect.
	c := newCache()
	c.set("k", &Info{}, 24*time.Hour)
	c.mu.Lock()
	defer c.mu.Unlock()
	assert.LessOrEqual(t, time.Until(c.m["k"].expireAt), cacheTTL)
}

func TestCacheIgnoresAnExpiredEntry(t *testing.T) {
	c := newCache()
	c.set("k", &Info{}, -time.Second)
	_, ok := c.get("k")
	assert.False(t, ok)
}

func TestCacheStaysBounded(t *testing.T) {
	// The keys come from the request, so a client that presents a new
	// credential per request must not grow the map without end.
	c := newCache()
	c.max = 10
	for i := 0; i < 1000; i++ {
		c.set(cacheKey("token", string(rune(i))), &Info{}, cacheTTL)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	assert.LessOrEqual(t, len(c.m), c.max)
}
