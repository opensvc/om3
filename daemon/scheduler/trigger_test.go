package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The first event runs the entry, the ones coming during the interval run it
// once at its end, and each interval ending with such a run doubles the next
// one, up to the max.
func TestTriggerBacksOff(t *testing.T) {
	g, err := newTrigger("@10s backoff @40s")
	require.NoError(t, err)
	t0 := time.Unix(0, 0)
	require.True(t, g.onEvent(t0))
	g.ran(t0, false)

	assert.False(t, g.onEvent(t0.Add(2*time.Second)))
	assert.Equal(t, t0.Add(10*time.Second), g.dueAt)
	assert.False(t, g.onEvent(t0.Add(3*time.Second)), "coalesced")
	g.ran(g.dueAt, true)
	assert.Equal(t, 20*time.Second, g.interval)

	t1 := t0.Add(10 * time.Second)
	assert.False(t, g.onEvent(t1.Add(time.Second)))
	assert.Equal(t, t1.Add(20*time.Second), g.dueAt)
	g.ran(g.dueAt, true)
	assert.Equal(t, 40*time.Second, g.interval)

	t2 := t1.Add(20 * time.Second)
	assert.False(t, g.onEvent(t2.Add(time.Second)))
	g.ran(g.dueAt, true)
	assert.Equal(t, 40*time.Second, g.interval, "up to the max")
}

// Each interval passing without an event halves the interval back.
func TestTriggerEases(t *testing.T) {
	g, err := newTrigger("@10s backoff @40s")
	require.NoError(t, err)
	t0 := time.Unix(0, 0)
	g.ran(t0, false)
	g.interval = 40 * time.Second

	// due at t0+40s, then a quiet 40s interval: halved to 20s
	assert.True(t, g.onEvent(t0.Add(80*time.Second)))
	assert.Equal(t, 20*time.Second, g.interval)
	g.ran(t0.Add(80*time.Second), false)

	// due at +20s, then a quiet 30s holding a 20s interval: back to 10s
	assert.True(t, g.onEvent(t0.Add(80*time.Second+50*time.Second)))
	assert.Equal(t, 10*time.Second, g.interval)
}

// Without backoff, the interval stays.
func TestTriggerWithoutBackoff(t *testing.T) {
	g, err := newTrigger("@10s")
	require.NoError(t, err)
	t0 := time.Unix(0, 0)
	g.ran(t0, false)
	assert.False(t, g.onEvent(t0.Add(time.Second)))
	g.ran(g.dueAt, true)
	assert.Equal(t, 10*time.Second, g.interval)
	assert.True(t, g.onEvent(t0.Add(20*time.Second)))
}
