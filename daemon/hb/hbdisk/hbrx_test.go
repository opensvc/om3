package hbdisk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A peer counts alive when it wrote its slot since the last read, whatever
// the clocks say, and only the first read, with nothing to compare with,
// judges the age of the write.
func TestIsNewWrite(t *testing.T) {
	r := &rx{timeout: 15 * time.Second, last: make(map[string]time.Time)}
	now := time.Now()

	assert.True(t, r.isNewWrite("n2", now.Add(-time.Second)), "a recent first write")
	assert.False(t, r.isNewWrite("n2", now.Add(-time.Second)), "the same write read again")
	assert.True(t, r.isNewWrite("n2", now), "a new write")

	assert.False(t, r.isNewWrite("n3", now.Add(-time.Hour)), "an old first write")
	assert.True(t, r.isNewWrite("n3", now.Add(-time.Hour+5*time.Second)), "a new write of a peer whose clock is late")

	ahead := now.Add(time.Hour)
	assert.True(t, r.isNewWrite("n4", ahead), "a first write of a peer whose clock is ahead")
	assert.False(t, r.isNewWrite("n4", ahead), "the last write of a peer whose clock is ahead is not new again")

	assert.False(t, r.isNewWrite("n2", now), "each peer has a last read of its own")
}
