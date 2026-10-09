package clusterlock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/daemon/api"
)

// TestDeadline pins that the deadline of the holder is counted on its own
// clock, from durations, whatever the clock of the node that granted the
// lock says the time is.
func TestDeadline(t *testing.T) {
	// The clock of the node speaking is an hour ahead of the holder's.
	speakerNow := time.Now().Add(time.Hour)
	sent := time.Now()
	received := sent.Add(100 * time.Millisecond)
	item := api.ClusterLock{
		AcquiredAt: speakerNow,
		ExpiresAt:  speakerNow.Add(30 * time.Second),
	}
	assert.Equal(t, sent.Add(30*time.Second), deadline(item, sent, received),
		"no time left said: the lease from the request on")

	// The request waited 20s for the lock: the time left said keeps it.
	received = sent.Add(20 * time.Second)
	left := "29.9s"
	item.ExpiresIn = &left
	assert.Equal(t, received.Add(29900*time.Millisecond-returnMargin), deadline(item, sent, received))

	// Granted at once, the lease from the request on is the later bound.
	received = sent.Add(10 * time.Millisecond)
	left = "29.99s"
	assert.Equal(t, sent.Add(30*time.Second), deadline(item, sent, received))
}
