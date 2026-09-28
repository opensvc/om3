package hbrelay

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A peer counts alive when the relay stored a message of it since the last
// read, whatever the clocks of the relay and this node say, and only the
// first read, with nothing to compare with, judges the age of the store.
func TestIsNewWrite(t *testing.T) {
	r := &rx{lastAt: make(map[string]time.Time)}
	r.timeout = 15 * time.Second
	now := time.Now()

	assert.True(t, r.isNewWrite("n2", now.Add(-time.Second)), "a recent first store")
	assert.False(t, r.isNewWrite("n2", now.Add(-time.Second)), "the same store read again")
	assert.True(t, r.isNewWrite("n2", now), "a new store")

	assert.False(t, r.isNewWrite("n3", now.Add(-time.Hour)), "an old first store")
	assert.True(t, r.isNewWrite("n3", now.Add(-time.Hour+5*time.Second)), "a new store by a relay whose clock is late")

	ahead := now.Add(time.Hour)
	assert.True(t, r.isNewWrite("n4", ahead), "a first store by a relay whose clock is ahead")
	assert.False(t, r.isNewWrite("n4", ahead), "the last store of a dead peer is not new again")

	assert.False(t, r.isNewWrite("n2", now), "each peer has a last read of its own")
}
