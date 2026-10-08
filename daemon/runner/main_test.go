package runner

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/priority"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// syncBuffer is a log destination the runner and the test goroutines write
// and read concurrently.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (t *syncBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.b.Write(p)
}

func (t *syncBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.b.String()
}

// TestTheActionsHeldBackByMaxRunningAreLogged verifies the runner says which
// action waits for a slot, and how long it waited, which is the only trace
// of max_parallel holding actions back.
func TestTheActionsHeldBackByMaxRunningAreLogged(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bus := pubsub.NewBus(t.Name())
	ctx = pubsub.ContextWithBus(ctx, bus)
	bus.Start(ctx)

	var logs syncBuffer
	r := New(pubsub.WithQueueSize(100))
	r.log = plog.NewLogger(zerolog.New(&logs))
	r.SetMaxRunning(1)
	r.SetInterval(10 * time.Millisecond)
	require.NoError(t, r.Start(ctx))
	defer func() { _ = r.Stop() }()

	release := make(chan struct{})
	started := make(chan struct{})
	firstDone := make(chan error)
	go func() {
		firstDone <- r.Run(priority.T(10), "svc1: starting", func() error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started
	assert.NotContains(t, logs.String(), "svc1: starting: queued", "a free slot holds nothing back")

	secondDone := make(chan error)
	go func() {
		secondDone <- r.Run(priority.T(10), "svc2: starting", func() error { return nil })
	}()
	require.Eventually(t, func() bool {
		return strings.Contains(logs.String(), "svc2: starting: queued behind 1 running (max 1), 1 waiting")
	}, time.Second, 10*time.Millisecond, logs.String())

	close(release)
	require.NoError(t, <-firstDone)
	require.NoError(t, <-secondDone)
	assert.Regexp(t, `svc2: starting: runs after waiting [0-9.]+m?s \(1 running, 0 waiting\)`, logs.String())
}
