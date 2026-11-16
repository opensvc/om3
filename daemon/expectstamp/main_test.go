package expectstamp

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// The stamp of a request is after every stamp seen, whatever the clock of
// the node accepting it.
func TestNext(t *testing.T) {
	now := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	assert.Equal(t, now, Next(now), "nothing seen")
	assert.Equal(t, now, Next(now, now.Add(-time.Second)), "the clock is after what was seen")
	ahead := now.Add(30 * time.Second)
	assert.Equal(t, ahead.Add(time.Nanosecond), Next(now, now.Add(-time.Second), ahead), "a peer clock ahead")
	assert.Equal(t, now.Add(time.Nanosecond), Next(now, now), "a stamp equal to the clock")
}

// Of two global expects stamped the same, every node keeps the one of the
// greater orchestration id.
func TestNewer(t *testing.T) {
	now := time.Now()
	a := uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	b := uuid.MustParse("00000000-0000-0000-0000-00000000000b")
	assert.True(t, Newer(now.Add(time.Nanosecond), a, now, b))
	assert.False(t, Newer(now, b, now.Add(time.Nanosecond), a))
	assert.True(t, Newer(now, b, now, a))
	assert.False(t, Newer(now, a, now, b))
	assert.False(t, Newer(now, a, now, a), "the same one")
	assert.False(t, Newer(now, a, now, uuid.Nil), "the same one, ended here: not adopted back")
	assert.False(t, Newer(now, uuid.Nil, now, a), "ended here, not yet there: converged")
}

// Of two peers stamped the same, the one running the request is adopted
// over the one that ended it.
func TestNewerCandidate(t *testing.T) {
	now := time.Now()
	a := uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	b := uuid.MustParse("00000000-0000-0000-0000-00000000000b")
	assert.True(t, NewerCandidate(now, a, now, uuid.Nil), "running over ended")
	assert.False(t, NewerCandidate(now, uuid.Nil, now, a), "ended under running")
	assert.False(t, NewerCandidate(now, uuid.Nil, now, uuid.Nil))
	assert.True(t, NewerCandidate(now, b, now, a))
	assert.True(t, NewerCandidate(now.Add(time.Nanosecond), uuid.Nil, now, a), "a later stamp wins")
}
