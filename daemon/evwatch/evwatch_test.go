package evwatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/plog"
)

// The count is of the resources, and goes back to zero once the instances
// holding them are unset.
func TestCounter(t *testing.T) {
	c := NewCounter()
	before, after := c.Set("ns1/svc/a", []string{"vm1", "vm2"})
	assert.Equal(t, 0, before)
	assert.Equal(t, 2, after)
	_, after = c.Set("ns1/svc/b", []string{"vm1", ""})
	assert.Equal(t, 4, after)
	assert.True(t, c.Has("vm2"))
	assert.Equal(t, []string{"vm1", "vm2"}, c.Keys(), "each name once, the unnamed left out")
	before, after = c.Set("ns1/svc/a", []string{"vm1"})
	assert.Equal(t, 4, before)
	assert.Equal(t, 3, after)
	assert.False(t, c.Has("vm2"))
	c.Unset("ns1/svc/b")
	_, after = c.Unset("ns1/svc/a")
	assert.Equal(t, 0, after)
	assert.Empty(t, c.Keys())
}

// The keys of a status are the info values of its resources of the types, a
// resource with none counted with an empty key.
func TestKeysOf(t *testing.T) {
	st := instance.Status{Resources: instance.ResourceStatuses{
		"container#1": {Type: "container.kvm", Info: map[string]any{"name": "vm1"}},
		"container#2": {Type: "container.kvm", Info: map[string]any{"name": 4}},
		"container#3": {Type: "container.podman", Info: map[string]any{"name": "c1"}},
		"disk#1":      {Type: "disk.drbd", Info: map[string]any{"res": "r1"}},
		"fs#1":        resource.Status{Type: "fs.ext4"},
	}}
	assert.Equal(t, []string{"", "vm1"}, KeysOf(st, "name", "container.kvm"))
	assert.Equal(t, []string{"c1", "vm1"}, KeysOf(st, "name", "container.podman", "container.kvm")[1:])
	assert.Equal(t, []string{"r1"}, KeysOf(st, "res", "disk.drbd"))
	assert.Empty(t, KeysOf(instance.Status{}, "name", "container.kvm"))
}

// Notifications within the delay make one call per key, and none after Stop.
func TestDebouncer(t *testing.T) {
	var mu sync.Mutex
	calls := make(map[string]int)
	d := NewDebouncer(50*time.Millisecond, func(key string) {
		mu.Lock()
		calls[key]++
		mu.Unlock()
	})
	for i := 0; i < 5; i++ {
		d.Notify("a")
		time.Sleep(10 * time.Millisecond)
	}
	d.Notify("b")
	time.Sleep(150 * time.Millisecond)
	mu.Lock()
	assert.Equal(t, map[string]int{"a": 1, "b": 1}, calls)
	mu.Unlock()
	d.Notify("c")
	d.Stop()
	d.Notify("d")
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	assert.Zero(t, calls["c"])
	assert.Zero(t, calls["d"])
	mu.Unlock()
}

func script(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "source.sh")
	require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755))
	return p
}

// The lines of the command are delivered, and the tail of its stderr says
// why it ended.
func TestStreamLines(t *testing.T) {
	var lines []string
	tail, err := StreamLines(context.Background(), []string{"EVWATCH_TEST=v"}, func(s string) { lines = append(lines, s) },
		script(t, `echo one; echo "two $EVWATCH_TEST"; echo oops >&2; exit 3`))
	assert.Error(t, err)
	assert.Equal(t, []string{"one", "two v"}, lines)
	assert.Equal(t, "oops", tail)
}

// A command stopped is sent a SIGTERM, and a child holding its output does
// not hold the wait past the wait delay.
func TestStreamLinesStopsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 1)
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	begin := time.Now()
	_, _ = StreamLines(ctx, nil, func(string) {
		select {
		case started <- struct{}{}:
		default:
		}
	}, script(t, `sleep 600 & echo up; trap 'exit 0' TERM; while :; do sleep 1; done`))
	assert.Less(t, time.Since(begin), streamWaitDelay+2*time.Second)
	select {
	case <-started:
	default:
		t.Fatal("no line read before the stop")
	}
}

// fakeSource counts its runs, and runs until its context is done, or ends at
// once when told to fail.
type fakeSource struct {
	runs      atomic.Int32
	checks    atomic.Int32
	mu        sync.Mutex
	checkErr  error
	failRuns  atomic.Bool
	ignoreCtx atomic.Bool
}

func (t *fakeSource) Name() string { return "fake" }

func (t *fakeSource) Check(context.Context) error {
	t.checks.Add(1)
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.checkErr
}

func (t *fakeSource) setCheckErr(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.checkErr = err
}

func (t *fakeSource) Run(ctx context.Context, ready func(), emit func(string)) error {
	t.runs.Add(1)
	if t.failRuns.Load() {
		return fmt.Errorf("stream ended")
	}
	ready()
	emit("vm1")
	<-ctx.Done()
	if t.ignoreCtx.Load() {
		time.Sleep(100 * time.Millisecond)
	}
	return ctx.Err()
}

func testPolicy() Policy {
	return Policy{
		StopGrace:    100 * time.Millisecond,
		RestartMin:   20 * time.Millisecond,
		RestartMax:   80 * time.Millisecond,
		QuickFailure: time.Second,
		CheckTimeout: time.Second,
	}
}

func waitState(t *testing.T, r *Runner, want State) {
	t.Helper()
	require.Eventually(t, func() bool { return r.State() == want }, 2*time.Second, 5*time.Millisecond, "state %s, want %s", r.State(), want)
}

// The source runs from the first resource on, once, and stops once none is
// left past the grace delay, a resource back within it keeping the same run.
func TestRunnerRunsOnDemand(t *testing.T) {
	src := &fakeSource{}
	var readies, events atomic.Int32
	r := NewRunner(src, testPolicy(), plog.NewDefaultLogger(), func() { readies.Add(1) }, func(string) { events.Add(1) })
	r.Start(context.Background())
	defer r.Stop()
	assert.Equal(t, StateIdle, r.State())

	r.SetDemand(1)
	waitState(t, r, StateRunning)
	r.SetDemand(2)
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, int32(1), src.runs.Load(), "a second resource runs no second source")
	assert.Eventually(t, func() bool { return readies.Load() == 1 && events.Load() == 1 }, time.Second, 5*time.Millisecond)

	r.SetDemand(0)
	waitState(t, r, StateDraining)
	r.SetDemand(1)
	waitState(t, r, StateRunning)
	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, StateRunning, r.State(), "a resource back within the grace keeps the run")
	assert.Equal(t, int32(1), src.runs.Load())

	r.SetDemand(0)
	waitState(t, r, StateIdle)
	r.SetDemand(1)
	waitState(t, r, StateRunning)
	assert.Equal(t, int32(2), src.runs.Load())
}

// A source that can not run is disabled until no resource is left, then
// checked again.
func TestRunnerDisablesAnUnsupportedSource(t *testing.T) {
	src := &fakeSource{}
	src.setCheckErr(fmt.Errorf("virsh is not installed: %w", ErrUnsupported))
	r := NewRunner(src, testPolicy(), plog.NewDefaultLogger(), nil, nil)
	r.Start(context.Background())
	defer r.Stop()
	r.SetDemand(1)
	waitState(t, r, StateDisabled)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), src.checks.Load(), "not checked again while disabled")
	r.SetDemand(0)
	waitState(t, r, StateIdle)
	src.setCheckErr(nil)
	r.SetDemand(1)
	waitState(t, r, StateRunning)
}

// A source ending runs again after a delay, and a policy limiting the quick
// failures disables it past the limit.
func TestRunnerRestartsAFailingSource(t *testing.T) {
	src := &fakeSource{}
	src.failRuns.Store(true)
	r := NewRunner(src, testPolicy(), plog.NewDefaultLogger(), nil, nil)
	r.Start(context.Background())
	defer r.Stop()
	r.SetDemand(1)
	require.Eventually(t, func() bool { return src.runs.Load() >= 4 }, 2*time.Second, 5*time.Millisecond, "never given up")
	r.Stop()

	src2 := &fakeSource{}
	src2.failRuns.Store(true)
	policy := testPolicy()
	policy.MaxQuickFailures = 3
	r2 := NewRunner(src2, policy, plog.NewDefaultLogger(), nil, nil)
	r2.Start(context.Background())
	defer r2.Stop()
	r2.SetDemand(1)
	waitState(t, r2, StateDisabled)
	assert.Equal(t, int32(3), src2.runs.Load())
	r2.SetDemand(0)
	waitState(t, r2, StateIdle)
	src2.failRuns.Store(false)
	r2.SetDemand(1)
	waitState(t, r2, StateRunning)
}

// A stop returns once the source stopped, in every state.
func TestRunnerStopWaitsForTheSource(t *testing.T) {
	src := &fakeSource{}
	src.ignoreCtx.Store(true)
	r := NewRunner(src, testPolicy(), plog.NewDefaultLogger(), nil, nil)
	r.Start(context.Background())
	r.SetDemand(1)
	waitState(t, r, StateRunning)
	begin := time.Now()
	r.Stop()
	assert.GreaterOrEqual(t, time.Since(begin), 100*time.Millisecond, "the source stopped before the stop returned")
	assert.Less(t, time.Since(begin), time.Second)

	idle := NewRunner(&fakeSource{}, testPolicy(), plog.NewDefaultLogger(), nil, nil)
	idle.Start(context.Background())
	idle.Stop()
	NewRunner(&fakeSource{}, testPolicy(), plog.NewDefaultLogger(), nil, nil).Stop()
}

// SetDemand never blocks, with no reader, the latest demand winning.
func TestSetDemandNeverBlocks(t *testing.T) {
	r := NewRunner(&fakeSource{}, testPolicy(), plog.NewDefaultLogger(), nil, nil)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			r.SetDemand(i)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SetDemand blocked")
	}
	assert.Equal(t, 99, <-r.demandC)
}

// Only a tool that ran to its end and printed no flag is unsupported: one
// timing out or not running is retried.
func TestProbeHelp(t *testing.T) {
	ok := script(t, `echo "  --loop   loop until interrupted"`)
	require.NoError(t, ProbeHelp(context.Background(), nil, "--loop", ok))

	noFlag := script(t, `echo "  --domain <string>"`)
	require.ErrorIs(t, ProbeHelp(context.Background(), nil, "--loop", noFlag), ErrUnsupported)

	noCommand := script(t, `echo "error: command 'event' doesn't exist" >&2; exit 1`)
	require.ErrorIs(t, ProbeHelp(context.Background(), nil, "--loop", noCommand), ErrUnsupported)

	slow := script(t, `sleep 5; echo "--loop"`)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := ProbeHelp(ctx, nil, "--loop", slow)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUnsupported, "a timeout says nothing of the tool")

	err = ProbeHelp(context.Background(), nil, "--loop", filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUnsupported, "a tool that could not run says nothing of it")
}
