package rescontainer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The wait for the shutdown of a container leaves StopKillMargin before the
// deadline of the action to force it, and is the stop_timeout otherwise.
func TestShutdownWait(t *testing.T) {
	assert.Equal(t, 20*time.Second, ShutdownWait(context.Background(), 20*time.Second), "no deadline")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	d := ShutdownWait(ctx, 20*time.Second)
	assert.True(t, d <= 15*time.Second && d > 14*time.Second, "the deadline as the stop_timeout: %s", d)
	assert.Equal(t, 5*time.Second, ShutdownWait(ctx, 5*time.Second), "a stop_timeout well within the deadline")

	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	assert.Zero(t, ShutdownWait(ctx, 20*time.Second), "no time left for a wait")
}
