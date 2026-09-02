package hbrelay

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

// posted collects what the loop handed to the relay.
type posted struct {
	mu sync.Mutex
	l  [][]byte

	// failing makes the relay refuse, as an unreachable or throttling
	// one does.
	failing    bool
	retryAfter time.Duration
}

func (p *posted) fail(v bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failing = v
}

func (p *posted) post(b []byte) (bool, time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.l = append(p.l, append([]byte(nil), b...))
	return !p.failing, p.retryAfter
}

func (p *posted) all() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([][]byte(nil), p.l...)
}

func (p *posted) len() int {
	return len(p.all())
}

func newTestTx(interval time.Duration) *tx {
	return &tx{cfg: cfg{
		id:       "hb#1.tx",
		interval: interval,
		relay:    "relay.example.com:3333",
		log:      plog.NewDefaultLogger(),
	}}
}

// runPostLoop drives the loop for the duration of the test and returns
// the channel to feed messages into.
func runPostLoop(t *testing.T, tr *tx, p *posted) chan []byte {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	msgC := make(chan []byte)
	done := make(chan struct{})
	go func() {
		defer close(done)
		tr.postLoop(ctx, msgC, nil, p.post)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return msgC
}

// TestPostLoopPostsABurstOnce is what this pacing is for: the daemon
// produces a message per propagation tick, the relay keeps only the
// last one, and posting each of them is work the relay carries for
// nothing.
func TestPostLoopPostsABurstOnce(t *testing.T) {
	tr := newTestTx(time.Hour)
	p := &posted{}
	msgC := runPostLoop(t, tr, p)

	for _, s := range []string{"first", "second", "third"} {
		msgC <- []byte(s)
	}
	// The loop reads and posts on the same goroutine, so a fourth send
	// that is read proves the first three were handled.
	msgC <- []byte("fourth")

	all := p.all()
	require.Len(t, all, 1, "the first message goes out, the rest wait for the tick")
	assert.Equal(t, "first", string(all[0]))
}

// TestPostLoopPostsTheFreshestMessage pins which of a burst is posted
// when the interval elapses: the last, since the others are superseded.
func TestPostLoopPostsTheFreshestMessage(t *testing.T) {
	tr := newTestTx(150 * time.Millisecond)
	p := &posted{}
	msgC := runPostLoop(t, tr, p)

	msgC <- []byte("first")
	for _, s := range []string{"second", "third"} {
		msgC <- []byte(s)
	}

	require.Eventually(t, func() bool { return p.len() >= 2 }, time.Second, 10*time.Millisecond)
	all := p.all()
	assert.Equal(t, "first", string(all[0]))
	assert.Equal(t, "third", string(all[1]), "the tick carries the freshest message, not the queued ones")
}

// TestPostLoopKeepsBeating covers the other half: a relay stream that
// stops posting is a peer that goes stale, so the last message is
// posted again every interval, refreshing the timestamp the peers read.
func TestPostLoopKeepsBeating(t *testing.T) {
	tr := newTestTx(100 * time.Millisecond)
	p := &posted{}
	msgC := runPostLoop(t, tr, p)

	msgC <- []byte("only")

	require.Eventually(t, func() bool { return p.len() >= 3 }, 2*time.Second, 10*time.Millisecond)
	for _, b := range p.all() {
		assert.Equal(t, "only", string(b))
	}
}

// TestPostLoopPostsAfterAQuietSpell pins that the pacing does not delay
// a change that follows an idle period: the node that just started, or
// the cluster that just changed, reaches its peers without waiting.
func TestPostLoopPostsAfterAQuietSpell(t *testing.T) {
	tr := newTestTx(50 * time.Millisecond)
	p := &posted{}
	msgC := runPostLoop(t, tr, p)

	msgC <- []byte("first")
	require.Eventually(t, func() bool { return p.len() >= 1 }, time.Second, 5*time.Millisecond)

	time.Sleep(80 * time.Millisecond)
	before := p.len()
	msgC <- []byte("after the quiet spell")
	require.Eventually(t, func() bool { return p.len() > before }, time.Second, 5*time.Millisecond)
	all := p.all()
	assert.Equal(t, "after the quiet spell", string(all[len(all)-1]))
}

func TestPostLoopPostsNothingUntilThereIsAMessage(t *testing.T) {
	tr := newTestTx(20 * time.Millisecond)
	p := &posted{}
	runPostLoop(t, tr, p)

	time.Sleep(100 * time.Millisecond)
	assert.Zero(t, p.len(), "an empty stream has nothing to beat with")
}

// TestPostLoopRetriesAFailedPost is what keeps a refused post from
// costing a whole beat: the peers only see this stream alive when the
// relay timestamps a post, so a failure is retried inside the interval
// rather than at the next one.
func TestPostLoopRetriesAFailedPost(t *testing.T) {
	tr := newTestTx(400 * time.Millisecond) // retryDelay is 1s, its floor
	p := &posted{}
	p.fail(true)
	msgC := runPostLoop(t, tr, p)

	msgC <- []byte("first")
	require.Eventually(t, func() bool { return p.len() >= 1 }, time.Second, 5*time.Millisecond)

	// The retry comes without a new message, and before the stream
	// would have posted three more times had it kept its interval.
	require.Eventually(t, func() bool { return p.len() >= 2 }, 2*time.Second, 10*time.Millisecond)
	assert.Less(t, p.len(), 4, "a failing relay is retried, not hammered")
}

// TestPostLoopDoesNotHammerAFailingRelay covers the other side of the
// retry: the daemon keeps producing messages while the relay is down,
// and each of them must not become an attempt.
func TestPostLoopDoesNotHammerAFailingRelay(t *testing.T) {
	tr := newTestTx(time.Hour) // retryDelay is 15 minutes
	p := &posted{}
	p.fail(true)
	msgC := runPostLoop(t, tr, p)

	for i := 0; i < 20; i++ {
		msgC <- []byte("change")
	}
	msgC <- []byte("last") // read, so the previous ones were handled

	assert.Equal(t, 1, p.len(), "the messages of an outage are one attempt, not twenty")
}

func TestRetryDelay(t *testing.T) {
	assert.Equal(t, 15*time.Second, newTestTx(time.Minute).retryDelay())
	assert.Equal(t, time.Second, newTestTx(2*time.Second).retryDelay(), "the floor keeps a short interval from retrying in a loop")
}

func TestRetryAfterOf(t *testing.T) {
	for name, tc := range map[string]struct {
		header string
		want   time.Duration
	}{
		"the delta seconds a limiter sends": {"5", 5 * time.Second},
		"no header at all":                  {"", 0},
		"an http date this client ignores":  {"Wed, 21 Oct 2026 07:28:00 GMT", 0},
		"a nonsense value":                  {"soon", 0},
		"zero":                              {"0", 0},
		"a negative value":                  {"-1", 0},
	} {
		t.Run(name, func(t *testing.T) {
			h := http.Header{}
			if tc.header != "" {
				h.Set("Retry-After", tc.header)
			}
			assert.Equal(t, tc.want, retryAfterOf(h))
		})
	}
}
