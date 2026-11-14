//go:build linux

package mdmon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/util/plog"
)

// fakeWait is a wait whose events the test fires, and which records the
// offset of the file at each wait: the source has to read the file again
// from its start after each event.
type fakeWait struct {
	fire    chan error
	mu      sync.Mutex
	offsets []int64
}

func newFakeWait() *fakeWait {
	return &fakeWait{fire: make(chan error, 10)}
}

// read reads the file as the source does, and takes the events fired so far
// as seen, as the kernel does on a read of /proc/mdstat.
func (t *fakeWait) read(fd int) ([]byte, error) {
	for {
		select {
		case <-t.fire:
		default:
			return readAll(fd)
		}
	}
}

func (t *fakeWait) wait(fd int, timeout time.Duration) (bool, error) {
	off, _ := unix.Seek(fd, 0, 1)
	t.mu.Lock()
	t.offsets = append(t.offsets, off)
	t.mu.Unlock()
	select {
	case err := <-t.fire:
		return err == nil, err
	case <-time.After(timeout):
		return false, nil
	}
}

// mdstat writes a fake /proc/mdstat and returns its path.
func mdstat(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "mdstat")
	require.NoError(t, os.WriteFile(p, []byte("Personalities : [raid1]\nunused devices: <none>\n"), 0o644))
	return p
}

// recorder counts the ready calls and the events emitted, and keeps their
// names.
type recorder struct {
	mu      sync.Mutex
	readies int
	events  int
	names   []string
}

func (t *recorder) ready() { t.mu.Lock(); t.readies++; t.mu.Unlock() }
func (t *recorder) emit(name string) {
	t.mu.Lock()
	t.events++
	t.names = append(t.names, name)
	t.mu.Unlock()
}
func (t *recorder) counts() (int, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.readies, t.events
}

func run(t *testing.T, src *source, rec *recorder) (context.CancelFunc, chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- src.Run(ctx, rec.ready, rec.emit) }()
	t.Cleanup(cancel)
	return cancel, done
}

// The source is ready once it read the file, emits each event, reading the
// file again from its start, and ends with its context.
func TestSourceRun(t *testing.T) {
	fw := newFakeWait()
	src := &source{log: plog.NewDefaultLogger(), path: mdstat(t), wait: fw.wait, read: fw.read}
	require.NoError(t, src.Check(context.Background()))
	rec := &recorder{}
	cancel, done := run(t, src, rec)
	require.Eventually(t, func() bool { r, _ := rec.counts(); return r == 1 }, 2*time.Second, 10*time.Millisecond)
	_, events := rec.counts()
	assert.Zero(t, events, "ready emits nothing")

	fw.fire <- nil
	require.Eventually(t, func() bool { _, e := rec.counts(); return e == 1 }, 2*time.Second, 10*time.Millisecond)
	fw.fire <- nil
	require.Eventually(t, func() bool { _, e := rec.counts(); return e == 2 }, 2*time.Second, 10*time.Millisecond)

	cancel()
	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("the source did not end with its context")
	}
	fw.mu.Lock()
	defer fw.mu.Unlock()
	for _, off := range fw.offsets {
		assert.NotZero(t, off, "each wait follows a read of the whole file")
	}
	readies, _ := rec.counts()
	assert.Equal(t, 1, readies)
}

// A file missing, as with the md module not loaded, is waited for: the
// source is ready once it exists.
func TestSourceWaitsForTheModule(t *testing.T) {
	saved := moduleCheckInterval
	moduleCheckInterval = 20 * time.Millisecond
	t.Cleanup(func() { moduleCheckInterval = saved })
	p := filepath.Join(t.TempDir(), "mdstat")
	src := &source{log: plog.NewDefaultLogger(), path: p, wait: newFakeWait().wait, read: readAll}
	require.NoError(t, src.Check(context.Background()), "a module not loaded is no reason to disable")
	rec := &recorder{}
	run(t, src, rec)
	time.Sleep(100 * time.Millisecond)
	readies, _ := rec.counts()
	require.Zero(t, readies)
	require.NoError(t, os.WriteFile(p, []byte("Personalities :\n"), 0o644))
	require.Eventually(t, func() bool { r, _ := rec.counts(); return r == 1 }, 2*time.Second, 10*time.Millisecond)
}

// The file removed, as by the md module unloaded, ends the source.
func TestSourceEndsWhenTheModuleIsUnloaded(t *testing.T) {
	saved := aliveCheckInterval
	aliveCheckInterval = 0
	t.Cleanup(func() { aliveCheckInterval = saved })
	p := mdstat(t)
	src := &source{log: plog.NewDefaultLogger(), path: p, read: readAll, wait: func(int, time.Duration) (bool, error) {
		time.Sleep(10 * time.Millisecond)
		return false, nil
	}}
	rec := &recorder{}
	_, done := run(t, src, rec)
	require.Eventually(t, func() bool { r, _ := rec.counts(); return r == 1 }, 2*time.Second, 10*time.Millisecond)
	require.NoError(t, os.Remove(p))
	select {
	case err := <-done:
		assert.ErrorContains(t, err, "unloaded")
	case <-time.After(2 * time.Second):
		t.Fatal("the source did not end")
	}
}

// A file signaling events for ever, as a module unloading, ends the source,
// and a wait failing too.
func TestSourceEndsOnASpinOrAWaitError(t *testing.T) {
	settle := eventSettle
	eventSettle = 0
	t.Cleanup(func() { eventSettle = settle })
	src := &source{log: plog.NewDefaultLogger(), path: mdstat(t), read: readAll, wait: func(int, time.Duration) (bool, error) { return true, nil }}
	rec := &recorder{}
	_, done := run(t, src, rec)
	select {
	case err := <-done:
		assert.ErrorContains(t, err, "events a second")
	case <-time.After(2 * time.Second):
		t.Fatal("the source did not end")
	}

	src = &source{log: plog.NewDefaultLogger(), path: mdstat(t), read: readAll, wait: func(int, time.Duration) (bool, error) { return false, errors.New("bad fd") }}
	_, done = run(t, src, &recorder{})
	select {
	case err := <-done:
		assert.ErrorContains(t, err, "bad fd")
	case <-time.After(2 * time.Second):
		t.Fatal("the source did not end")
	}
}

// On the /proc/mdstat of the node, when it has one, the source gets ready,
// signals no event of its own reading, and ends with its context.
func TestSourceOnProcMdstat(t *testing.T) {
	if _, err := os.Stat(procMdstat); err != nil {
		t.Skip("no /proc/mdstat")
	}
	src := newSource(plog.NewDefaultLogger())
	rec := &recorder{}
	cancel, done := run(t, src, rec)
	require.Eventually(t, func() bool { r, _ := rec.counts(); return r == 1 }, 2*time.Second, 10*time.Millisecond)
	time.Sleep(time.Second)
	_, events := rec.counts()
	assert.Zero(t, events, "no md event of its own reads")
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("the source did not end with its context")
	}
}

// An event names the arrays whose lines changed since the previous read, and
// names none when no line changed, as an array resynced whole between two
// reads. The progress of a resync is no change.
func TestSourceNamesTheChangedArrays(t *testing.T) {
	fw := newFakeWait()
	p := filepath.Join(t.TempDir(), "mdstat")
	require.NoError(t, os.WriteFile(p, []byte(mdstatIdle), 0o644))
	src := &source{log: plog.NewDefaultLogger(), path: p, wait: fw.wait, read: fw.read}
	rec := &recorder{}
	run(t, src, rec)
	require.Eventually(t, func() bool { r, _ := rec.counts(); return r == 1 }, 2*time.Second, 10*time.Millisecond)

	step := func(content string, want ...string) {
		t.Helper()
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		rec.mu.Lock()
		rec.names = nil
		rec.mu.Unlock()
		fw.fire <- nil
		require.Eventually(t, func() bool {
			rec.mu.Lock()
			defer rec.mu.Unlock()
			return len(rec.names) == len(want)
		}, 2*time.Second, 10*time.Millisecond)
		rec.mu.Lock()
		defer rec.mu.Unlock()
		assert.Equal(t, want, rec.names)
	}
	step(mdstatResyncing, "md126", "md127")
	step(mdstatResyncFurther, "")
	step(mdstatFailed, "md126", "md127")
	step(mdstatFailed, "")
}

// A file larger than a read is read whole.
func TestReadAllReadsTheWholeFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "mdstat")
	content := make([]byte, 10000)
	for i := range content {
		content[i] = 'x'
	}
	require.NoError(t, os.WriteFile(p, content, 0o644))
	fd, err := unix.Open(p, unix.O_RDONLY, 0)
	require.NoError(t, err)
	defer unix.Close(fd)
	b, err := readAll(fd)
	require.NoError(t, err)
	assert.Len(t, b, 10000)
}

// The events signaled for one transition, as the two of the start of a
// check, are read once: the change names its array, and no event finding no
// change names every array.
func TestSourceReadsTheEventsOfATransitionOnce(t *testing.T) {
	fw := newFakeWait()
	p := filepath.Join(t.TempDir(), "mdstat")
	require.NoError(t, os.WriteFile(p, []byte(mdstatIdle), 0o644))
	src := &source{log: plog.NewDefaultLogger(), path: p, wait: fw.wait, read: fw.read}
	rec := &recorder{}
	run(t, src, rec)
	require.Eventually(t, func() bool { r, _ := rec.counts(); return r == 1 }, 2*time.Second, 10*time.Millisecond)
	require.NoError(t, os.WriteFile(p, []byte(mdstatFailed), 0o644))
	fw.fire <- nil
	fw.fire <- nil
	time.Sleep(eventSettle + 500*time.Millisecond)
	rec.mu.Lock()
	defer rec.mu.Unlock()
	assert.Equal(t, []string{"md126"}, rec.names)
}
