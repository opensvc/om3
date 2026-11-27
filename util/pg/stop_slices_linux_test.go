//go:build linux

package pg

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stopEvents records what happened of the stop of slices, in its order.
type stopEvents struct {
	sync.Mutex
	l []string
}

func (t *stopEvents) add(s string) {
	t.Lock()
	defer t.Unlock()
	t.l = append(t.l, s)
}

func (t *stopEvents) list() []string {
	t.Lock()
	defer t.Unlock()
	return append([]string{}, t.l...)
}

// fakeStopJobs has stopUnit wait with stopUnitJob for the job and the state
// given, in place of the ones of systemd, and records the stops asked with
// how they were: waited for or not.
func fakeStopJobs(t *testing.T, events *stopEvents, job func(context.Context, chan<- string) error, state func(context.Context) (string, error)) {
	t.Helper()
	savedStopUnit, savedPoll, savedTimeout := stopUnit, unitStopPoll, stopSlicesTimeout
	t.Cleanup(func() { stopUnit, unitStopPoll, stopSlicesTimeout = savedStopUnit, savedPoll, savedTimeout })
	unitStopPoll = 5 * time.Millisecond
	stopUnit = func(ctx context.Context, unit string, wait bool) error {
		if !wait {
			events.add("stop " + unit + " unwaited")
			return ctx.Err()
		}
		events.add("stop " + unit)
		return stopUnitJob(ctx, unit, job, state)
	}
}

// The stop of a slice returns once systemd has run its job, not once it has
// queued it: a start following the stop found the slice still loaded, gave
// it its properties, and lost the groups to the stop running after it.
func TestStopSlicesWaitsForTheStopJob(t *testing.T) {
	newFakeNode(t, false)
	events := &stopEvents{}
	job := func(_ context.Context, done chan<- string) error {
		go func() {
			time.Sleep(100 * time.Millisecond)
			events.add("done")
			done <- "done"
		}()
		return nil
	}
	state := func(context.Context) (string, error) { return "active", nil }
	fakeStopJobs(t, events, job, state)

	require.NoError(t, Config{ID: testObjSlice}.stopSlices([]string{testObjUnit}))
	events.add("returned")
	assert.Equal(t, []string{"stop " + testObjUnit, "done", "returned"}, events.list())
}

// A systemd not signaling the end of the job, as on a bus it sends no signal
// to, has the state of the slice tell it: inactive, or no longer loaded.
func TestStopSlicesWaitsForTheSliceToBeInactiveWithNoJobSignal(t *testing.T) {
	for name, last := range map[string]func() (string, error){
		"inactive":   func() (string, error) { return "inactive", nil },
		"failed":     func() (string, error) { return "failed", nil },
		"not loaded": func() (string, error) { return "", errNotLoaded(testObjUnit) },
	} {
		t.Run(name, func(t *testing.T) {
			newFakeNode(t, false)
			events := &stopEvents{}
			job := func(context.Context, chan<- string) error { return nil }
			reads := 0
			state := func(context.Context) (string, error) {
				reads++
				if reads < 4 {
					events.add("active")
					return "active", nil
				}
				events.add("stopped")
				return last()
			}
			fakeStopJobs(t, events, job, state)

			require.NoError(t, Config{ID: testObjSlice}.stopSlices([]string{testObjUnit}))
			events.add("returned")
			assert.Equal(t, []string{"stop " + testObjUnit, "active", "active", "active", "stopped", "returned"}, events.list())
		})
	}
}

// A stop systemd never runs is waited for until the deadline, and is no
// error: its job is queued, as it was before the stops were waited for. The
// slices left are then stopped without waiting, in a time of their own.
func TestStopSlicesReturnsAtTheDeadlineOfAStopNeverDone(t *testing.T) {
	newFakeNode(t, false)
	events := &stopEvents{}
	job := func(context.Context, chan<- string) error { return nil }
	state := func(context.Context) (string, error) { return "deactivating", nil }
	fakeStopJobs(t, events, job, state)
	stopSlicesTimeout = 100 * time.Millisecond

	began := time.Now()
	require.NoError(t, Config{ID: testObjSlice}.stopSlices([]string{testObjUnit, "opensvc-ns.t-svc.s-app.1.slice"}))
	spent := time.Since(began)
	assert.GreaterOrEqual(t, spent, stopSlicesTimeout, "the stop is waited for until the deadline")
	assert.Less(t, spent, 10*stopSlicesTimeout, "and no longer")
	assert.Equal(t, []string{
		"stop opensvc-ns.t-svc.s-app.1.slice",
		"stop " + testObjUnit + " unwaited",
	}, events.list(), "the nested slice first, then the other with no wait and a context not ended")
}

// A stop systemd refuses is an error, and is not waited for.
func TestStopSlicesReportsAStopRefused(t *testing.T) {
	newFakeNode(t, false)
	events := &stopEvents{}
	job := func(context.Context, chan<- string) error { return assert.AnError }
	state := func(context.Context) (string, error) { return "active", nil }
	fakeStopJobs(t, events, job, state)

	err := Config{ID: testObjSlice}.stopSlices([]string{testObjUnit})
	require.ErrorIs(t, err, assert.AnError)
	assert.NotErrorIs(t, err, errUnitStopPending)
}
