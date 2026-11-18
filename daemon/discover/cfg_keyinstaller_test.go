package discover

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
)

// fakeRefresh records the refreshes a keyInstaller runs. A refresh holds its
// slot until released, when hold is set.
type fakeRefresh struct {
	mu      sync.Mutex
	runs    []naming.Path
	active  int
	maxSeen int
	hold    bool
	release chan struct{}
	skip    func(naming.Path) bool
}

func newFakeRefresh(hold bool) *fakeRefresh {
	return &fakeRefresh{hold: hold, release: make(chan struct{}, 1000)}
}

func (f *fakeRefresh) refresh(ctx context.Context, p naming.Path, starting func()) {
	if f.skip != nil && f.skip(p) {
		return
	}
	starting()
	f.mu.Lock()
	f.runs = append(f.runs, p)
	f.active++
	if f.active > f.maxSeen {
		f.maxSeen = f.active
	}
	f.mu.Unlock()
	if f.hold {
		select {
		case <-f.release:
		case <-ctx.Done():
		}
	}
	f.mu.Lock()
	f.active--
	f.mu.Unlock()
}

func (f *fakeRefresh) state() (runs []naming.Path, active, maxSeen int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]naming.Path{}, f.runs...), f.active, f.maxSeen
}

func (f *fakeRefresh) waitActive(t *testing.T, n int) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, active, _ := f.state()
		return active == n
	}, 5*time.Second, time.Millisecond, "%d refreshes running", n)
}

func sec(i int) naming.Path {
	return naming.Path{Namespace: "system", Kind: naming.KindSec, Name: fmt.Sprintf("s%03d", i)}
}

func (ki *keyInstaller) idle() bool {
	ki.mu.Lock()
	defer ki.mu.Unlock()
	return ki.submitted == 0 && len(ki.queue) == 0 && len(ki.waiting) == 0 && len(ki.running) == 0 && len(ki.again) == 0
}

// Many datastores fetched at once run at most max parallel refreshes at a
// time, each once, in the order they were asked.
func TestKeyInstallerBoundsTheRefreshes(t *testing.T) {
	f := newFakeRefresh(true)
	ki := newKeyInstaller(context.Background(), func() int { return 3 }, f.refresh)
	for i := 0; i < 100; i++ {
		ki.request(sec(i))
	}
	for i := 0; i < 100; i++ {
		f.waitActive(t, min(3, 100-i))
		f.release <- struct{}{}
	}
	require.Eventually(t, ki.idle, 5*time.Second, time.Millisecond)
	runs, _, maxSeen := f.state()
	assert.Equal(t, 3, maxSeen)
	require.Len(t, runs, 100)
	seen := make(map[naming.Path]bool)
	for _, p := range runs {
		assert.False(t, seen[p], "%s refreshed once", p)
		seen[p] = true
	}
}

// With one slot, the refreshes run in the order they were asked.
func TestKeyInstallerRunsInOrder(t *testing.T) {
	f := newFakeRefresh(true)
	ki := newKeyInstaller(context.Background(), func() int { return 1 }, f.refresh)
	for i := 0; i < 10; i++ {
		ki.request(sec(i))
	}
	for i := 0; i < 10; i++ {
		f.waitActive(t, 1)
		f.release <- struct{}{}
	}
	require.Eventually(t, ki.idle, 5*time.Second, time.Millisecond)
	runs, _, _ := f.state()
	for i, p := range runs {
		assert.Equal(t, sec(i), p)
	}
}

// A change of max parallel applies to the next refreshes submitted.
func TestKeyInstallerFollowsMaxParallel(t *testing.T) {
	f := newFakeRefresh(true)
	var maxParallel atomic.Int32
	maxParallel.Store(1)
	ki := newKeyInstaller(context.Background(), func() int { return int(maxParallel.Load()) }, f.refresh)
	for i := 0; i < 10; i++ {
		ki.request(sec(i))
	}
	f.waitActive(t, 1)
	maxParallel.Store(4)
	f.release <- struct{}{}
	f.waitActive(t, 4)
	for i := 0; i < 9; i++ {
		f.release <- struct{}{}
	}
	require.Eventually(t, ki.idle, 5*time.Second, time.Millisecond)
	_, _, maxSeen := f.state()
	assert.Equal(t, 4, maxSeen)
}

// A datastore asked again while its refresh waits, in the queue or in the
// runner, is refreshed once: the waiting refresh reads the newest
// configuration.
func TestKeyInstallerCoalescesTheWaitingRefreshes(t *testing.T) {
	f := newFakeRefresh(true)
	ki := newKeyInstaller(context.Background(), func() int { return 1 }, f.refresh)
	ki.request(sec(0))
	f.waitActive(t, 1)
	for i := 0; i < 5; i++ {
		ki.request(sec(1))
	}
	f.release <- struct{}{}
	f.waitActive(t, 1)
	f.release <- struct{}{}
	require.Eventually(t, ki.idle, 5*time.Second, time.Millisecond)
	runs, _, _ := f.state()
	assert.Equal(t, []naming.Path{sec(0), sec(1)}, runs)
}

// A refresh submitted and not started yet, as one waiting for a runner
// slot, is still the waiting one.
func TestKeyInstallerCoalescesASubmittedRefresh(t *testing.T) {
	gate := make(chan struct{})
	var runs atomic.Int32
	ki := newKeyInstaller(context.Background(), func() int { return 1 }, func(ctx context.Context, p naming.Path, starting func()) {
		<-gate // waiting for its runner slot
		starting()
		runs.Add(1)
	})
	ki.request(sec(0))
	ki.request(sec(0))
	ki.request(sec(0))
	close(gate)
	require.Eventually(t, ki.idle, 5*time.Second, time.Millisecond)
	assert.Equal(t, int32(1), runs.Load())
}

// A datastore asked again while its refresh runs is refreshed once more,
// after the datastores already waiting.
func TestKeyInstallerQueuesARunningRefreshAgainAtTheTail(t *testing.T) {
	f := newFakeRefresh(true)
	ki := newKeyInstaller(context.Background(), func() int { return 1 }, f.refresh)
	ki.request(sec(0))
	f.waitActive(t, 1)
	ki.request(sec(1))
	for i := 0; i < 5; i++ {
		ki.request(sec(0))
	}
	for i := 0; i < 3; i++ {
		f.waitActive(t, 1)
		f.release <- struct{}{}
	}
	require.Eventually(t, ki.idle, 5*time.Second, time.Millisecond)
	runs, _, _ := f.state()
	assert.Equal(t, []naming.Path{sec(0), sec(1), sec(0)}, runs)
}

// A refresh skipped, no local instance using the datastore, frees its slot
// and leaves nothing waiting.
func TestKeyInstallerSkippedRefresh(t *testing.T) {
	f := newFakeRefresh(false)
	f.skip = func(p naming.Path) bool { return p == sec(0) }
	ki := newKeyInstaller(context.Background(), func() int { return 1 }, f.refresh)
	ki.request(sec(0))
	ki.request(sec(1))
	require.Eventually(t, ki.idle, 5*time.Second, time.Millisecond)
	runs, _, _ := f.state()
	assert.Equal(t, []naming.Path{sec(1)}, runs)

	ki.request(sec(2))
	require.Eventually(t, ki.idle, 5*time.Second, time.Millisecond)
	runs, _, _ = f.state()
	assert.Equal(t, []naming.Path{sec(1), sec(2)}, runs, "a refresh asked once idle runs")
}

// Once the daemon stops, the queued refreshes are dropped, and no other is
// submitted.
func TestKeyInstallerStops(t *testing.T) {
	f := newFakeRefresh(true)
	ctx, cancel := context.WithCancel(context.Background())
	ki := newKeyInstaller(ctx, func() int { return 2 }, f.refresh)
	for i := 0; i < 10; i++ {
		ki.request(sec(i))
	}
	f.waitActive(t, 2)
	cancel()
	require.Eventually(t, ki.idle, 5*time.Second, time.Millisecond)
	ki.request(sec(20))
	require.Eventually(t, ki.idle, 5*time.Second, time.Millisecond)
	runs, _, _ := f.state()
	assert.Len(t, runs, 2)
}
