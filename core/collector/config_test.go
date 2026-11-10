package collector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// collector.timeout is used within its documented bounds, its default when
// not set.
func TestBoundTimeout(t *testing.T) {
	assert.Equal(t, DefaultTimeout, BoundTimeout(0))
	assert.Equal(t, time.Second, BoundTimeout(100*time.Millisecond))
	assert.Equal(t, 8*time.Second, BoundTimeout(8*time.Second))
	assert.Equal(t, 20*time.Second, BoundTimeout(time.Minute))
}

// collector.speaker_grace_period is 1s at least, its default when not set.
func TestBoundSpeakerGracePeriod(t *testing.T) {
	assert.Equal(t, DefaultSpeakerGracePeriod, BoundSpeakerGracePeriod(0))
	assert.Equal(t, DefaultSpeakerGracePeriod, BoundSpeakerGracePeriod(-time.Second))
	assert.Equal(t, MinSpeakerGracePeriod, BoundSpeakerGracePeriod(500*time.Millisecond))
	assert.Equal(t, time.Second, BoundSpeakerGracePeriod(time.Second))
	assert.Equal(t, 2*time.Minute, BoundSpeakerGracePeriod(2*time.Minute))
}
