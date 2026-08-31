package hbdedup

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeenIsFalseUntilDelivered(t *testing.T) {
	c := NewCache(DefaultWindow)
	frame := []byte("an encrypted frame")
	key := NewKey(frame)

	_, ok := c.Seen(key)
	assert.False(t, ok, "an undelivered frame must be decoded")

	c.Delivered(key, "node2")

	nodename, ok := c.Seen(key)
	assert.True(t, ok)
	assert.Equal(t, "node2", nodename, "the nodename is what lets the second link report the peer alive without decrypting")
}

func TestDistinctFramesDoNotShareAKey(t *testing.T) {
	c := NewCache(DefaultWindow)
	c.Delivered(NewKey([]byte("frame one")), "node2")

	_, ok := c.Seen(NewKey([]byte("frame two")))
	assert.False(t, ok)

	// same length, different content
	_, ok = c.Seen(NewKey([]byte("frame ONE")))
	assert.False(t, ok)

	// same prefix, different length
	_, ok = c.Seen(NewKey([]byte("frame one ")))
	assert.False(t, ok)
}

// TestExpiry checks the generational rotation drops the old frames, so that a
// daemon running for months does not remember every heartbeat it received.
func TestExpiry(t *testing.T) {
	window := 20 * time.Millisecond
	c := NewCache(window)

	old := NewKey([]byte("old frame"))
	c.Delivered(old, "node2")
	_, ok := c.Seen(old)
	require.True(t, ok)

	// one window: the entry moves to the previous generation, still seen
	time.Sleep(window + 5*time.Millisecond)
	c.Delivered(NewKey([]byte("frame of generation 2")), "node2")
	_, ok = c.Seen(old)
	assert.True(t, ok, "a frame must survive one rotation, or a slow link would decode it again")

	// two windows: the entry is dropped with the generation it was in
	time.Sleep(window + 5*time.Millisecond)
	c.Delivered(NewKey([]byte("frame of generation 3")), "node2")
	_, ok = c.Seen(old)
	assert.False(t, ok)
	assert.Equal(t, 2, c.Len(), "only the last two generations are held")
}

func TestZeroWindowFallsBackToDefault(t *testing.T) {
	assert.Equal(t, DefaultWindow, NewCache(0).window)
	assert.Equal(t, DefaultWindow, NewCache(-time.Second).window)
}

// TestNilCacheDecodesEverything checks the behaviour a context carrying no
// cache falls back to, which is the one that predates the dedup.
func TestNilCacheDecodesEverything(t *testing.T) {
	var c *Cache
	key := NewKey([]byte("a frame"))

	assert.NotPanics(t, func() { c.Delivered(key, "node2") })
	_, ok := c.Seen(key)
	assert.False(t, ok, "a nil cache remembers nothing, so every frame is decoded")
	assert.Equal(t, 0, c.Len())
}

func TestCacheFromContext(t *testing.T) {
	assert.Nil(t, CacheFromContext(context.Background()), "a context with no cache yields a usable nil cache")

	c := NewCache(DefaultWindow)
	assert.Same(t, c, CacheFromContext(ContextWithCache(context.Background(), c)))
}

func TestCacheIsConcurrencySafe(t *testing.T) {
	c := NewCache(5 * time.Millisecond)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for n := 0; n < 200; n++ {
				key := NewKey([]byte(fmt.Sprintf("frame %d", n)))
				if _, ok := c.Seen(key); !ok {
					c.Delivered(key, fmt.Sprintf("node%d", i))
				}
			}
		}(i)
	}
	wg.Wait()
}

func BenchmarkNewKey(b *testing.B) {
	frame := make([]byte, 100*1024)
	b.SetBytes(int64(len(frame)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewKey(frame)
	}
}

// BenchmarkSingleLinkOverhead is what the cache costs per frame on a cluster
// running a single heartbeat, where no other link will ever ask about it: the
// hash, the lookup miss and the record, all of it wasted.
//
// It is the number to weigh a shortpath against. Skipping the cache when a
// single rx driver is registered would have to track a count the janitor
// changes on a config update or a hb component stop, in the receive path, to
// save this.
func BenchmarkSingleLinkOverhead(b *testing.B) {
	for _, size := range []int{4 * 1024, 32 * 1024, 256 * 1024} {
		b.Run(fmt.Sprintf("%dKB", size/1024), func(b *testing.B) {
			frame := make([]byte, size)
			for i := range frame {
				frame[i] = byte(i)
			}
			c := NewCache(DefaultWindow)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// a distinct frame every round, as a single link sees
				frame[0], frame[1] = byte(i), byte(i>>8)
				key := NewKey(frame)
				if _, ok := c.Seen(key); !ok {
					c.Delivered(key, "node2")
				}
			}
		})
	}
}
