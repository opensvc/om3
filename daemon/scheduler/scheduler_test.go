package scheduler

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/testhelper"
)

// testScheduler is a scheduler whose clock the test sets, and whose runs it
// records.
type testScheduler struct {
	*T
	clock time.Time
	runs  chan schedule.Entry
}

func newTestScheduler(t *testing.T) *testScheduler {
	t.Helper()
	testhelper.Setup(t)
	s := &testScheduler{
		T:     New(nil),
		clock: time.Date(2026, 9, 30, 10, 0, 0, 0, time.Local),
		runs:  make(chan schedule.Entry, 100),
	}
	// The runs are waited for, as a run records its last success on its
	// own, and the next test sets up another root.
	t.Cleanup(s.running.Wait)
	s.enabled = true
	s.isCollectorJoinable = true
	s.now = func() time.Time { return s.clock }
	s.exec = func(e schedule.Entry) error {
		s.runs <- e
		return nil
	}
	return s
}

// entry declares a schedule entry to the scheduler, as the configuration of
// the node or of an object does, and returns it.
func (s *testScheduler) entry(path naming.Path, key, sched string) schedule.Entry {
	e := schedule.Entry{
		Node: s.localhost,
		Path: path,
		Config: schedule.Config{
			Action:       "status",
			Key:          key,
			Schedule:     sched,
			StatefileKey: key,
		},
	}
	s.schedules.Add(path, e)
	return e
}

func (s *testScheduler) job(t *testing.T, e schedule.Entry) *job {
	t.Helper()
	j, ok := s.jobs.get(e.Path, e.Key)
	require.True(t, ok, "job %s is not planned", e.Key)
	require.GreaterOrEqual(t, j.index, 0, "job %s is known but out of the queue", e.Key)
	return j
}

// ranCount waits for the runs started in the background, and returns how
// many there were.
func (s *testScheduler) ranCount() int {
	s.running.Wait()
	return len(s.runs)
}

var testPath = naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "sched"}

// A job skipped when it is due is planned for its next period: whatever the
// reason to skip it, it runs again once that reason is gone. It used to be
// left known and unplanned, so it never ran again, and the scheduling that
// follows the events did not plan it either, as it was known.
func TestSkippedJobIsPlannedAgain(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testScheduler, *schedule.Entry)
	}{
		{"collector not joinable", func(s *testScheduler, e *schedule.Entry) {
			e.RequireCollector = true
			s.schedules.Add(e.Path, *e)
			s.isCollectorJoinable = false
		}},
		{"requirement not met", func(s *testScheduler, e *schedule.Entry) {
			e.Require = "fs#1(up)"
			s.schedules.Add(e.Path, *e)
			s.reqSatisfied.Set(e.Path, e.Key, errors.New("resource fs#1 status is down, required up"))
		}},
		{"not provisioned", func(s *testScheduler, e *schedule.Entry) {
			e.RequireProvisioned = true
			s.schedules.Add(e.Path, *e)
			s.provisioned[e.Path] = false
		}},
		{"not the replication source", func(s *testScheduler, e *schedule.Entry) {
			e.RequireReplicationSource = true
			s.schedules.Add(e.Path, *e)
			s.notSource[e.Path] = "the instance is not up"
		}},
		{"orchestration in progress", func(s *testScheduler, e *schedule.Entry) {
			e.RequireReplicationSource = true
			s.schedules.Add(e.Path, *e)
			s.notSource[e.Path] = ""
			instance.MonitorData.Set(e.Path, s.localhost, &instance.Monitor{GlobalExpect: instance.MonitorGlobalExpectStopped})
			t.Cleanup(func() { instance.MonitorData.Unset(e.Path, s.localhost) })
		}},
		{"node draining", func(s *testScheduler, e *schedule.Entry) {
			e.RequireReplicationSource = true
			s.schedules.Add(e.Path, *e)
			s.notSource[e.Path] = ""
			node.MonitorData.Set(s.localhost, &node.Monitor{LocalExpect: node.MonitorLocalExpectDrained})
			t.Cleanup(func() { node.MonitorData.Unset(s.localhost) })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestScheduler(t)
			e := s.entry(testPath, "sync#1.schedule", "@10m")
			s.planJob(e, s.clock)
			due := s.job(t, e).entry.NextRunAt

			tc.setup(s, &e)
			s.clock = due
			s.onTick()

			require.Equal(t, 0, s.ranCount(), "a skipped job does not run")
			assert.Equal(t, due.Add(10*time.Minute), s.job(t, e).entry.NextRunAt, "the next period is planned")
		})
	}
}

// A job due runs once, and its next run is planned from the time it was due,
// not from the time it was taken, so its runs do not drift.
func TestDueJobRunsAndDoesNotDrift(t *testing.T) {
	s := newTestScheduler(t)
	e := s.entry(naming.Path{}, "asset.schedule", "@10m")
	s.planJob(e, s.clock)
	due := s.job(t, e).entry.NextRunAt

	s.clock = due.Add(2 * time.Second)
	s.onTick()

	require.Equal(t, 1, s.ranCount())
	j := s.job(t, e)
	assert.Equal(t, due, j.entry.LastRunAt)
	assert.Equal(t, due.Add(10*time.Minute), j.entry.NextRunAt)
	assert.True(t, e.GetLastRun().Equal(due), "the last run is recorded: %s", e.GetLastRun())
}

// A job taken late, after the node was suspended for example, runs once, and
// its next run is planned from the time it was taken: the runs it missed are
// not made up in a burst.
func TestLateJobIsNotCaughtUp(t *testing.T) {
	s := newTestScheduler(t)
	e := s.entry(naming.Path{}, "asset.schedule", "@10m")
	s.planJob(e, s.clock)
	due := s.job(t, e).entry.NextRunAt

	s.clock = due.Add(35 * time.Minute)
	s.onTick()
	require.Equal(t, 1, s.ranCount())
	assert.Equal(t, s.clock.Add(10*time.Minute), s.job(t, e).entry.NextRunAt)

	// Nothing else is due now.
	s.onTick()
	assert.Equal(t, 1, s.ranCount())
}

// A job whose schedule was deleted is unplanned when due, not run.
func TestJobOfADeletedScheduleIsUnplanned(t *testing.T) {
	s := newTestScheduler(t)
	e := s.entry(naming.Path{}, "asset.schedule", "@10m")
	s.planJob(e, s.clock)
	s.schedules.Del(e.Path, e.Key)

	s.clock = s.job(t, e).entry.NextRunAt
	s.onTick()

	assert.Equal(t, 0, s.ranCount())
	assert.False(t, s.jobs.has(e.Path, e.Key))
	assert.Nil(t, s.jobs.first())
}

// The loop sleeps until the soonest job is due, and looks at the clock at
// least every maxSleep, so a job due at a wall time is taken within maxSleep
// of it whatever happened to the clock.
func TestNextWake(t *testing.T) {
	s := newTestScheduler(t)
	_, ok := s.nextWake(s.clock)
	assert.False(t, ok, "no job, no wake")

	soon := s.entry(naming.Path{}, "a.schedule", "@10m")
	soon.LastRunAt = s.clock.Add(-10*time.Minute + 30*time.Second)
	s.planJob(soon, s.clock)
	d, ok := s.nextWake(s.clock)
	require.True(t, ok)
	assert.Equal(t, 30*time.Second, d)

	s.jobs.del(soon.Path, soon.Key)
	later := s.entry(naming.Path{}, "b.schedule", "@2h")
	later.LastRunAt = s.clock
	s.planJob(later, s.clock)
	d, _ = s.nextWake(s.clock)
	assert.Equal(t, maxSleep, d)

	d, _ = s.nextWake(s.clock.Add(3 * time.Hour))
	assert.Equal(t, time.Duration(0), d, "a job due in the past is due now")
}

// A job whose schedule has not run yet is due at once.
func TestNeverRunJobIsDueNow(t *testing.T) {
	s := newTestScheduler(t)
	e := s.entry(naming.Path{}, "asset.schedule", "@10m")
	s.planJob(e, s.clock)
	assert.False(t, s.job(t, e).entry.NextRunAt.After(s.clock))
}

// The scheduling that follows the events plans a job when nothing blocks it,
// unplans it when something does, plans it again when that is gone, plans it
// again when its schedule changes, keeping its last run, and unplans it and
// forgets its schedule when it is no longer configured.
func TestScheduleEntries(t *testing.T) {
	s := newTestScheduler(t)
	config := schedule.Config{Action: "pushasset", Key: "asset.schedule", Schedule: "@10m", StatefileKey: "asset", RequireCollector: true}
	path := naming.Path{}

	s.isCollectorJoinable = false
	s.scheduleEntries(path, []schedule.Config{config})
	assert.False(t, s.jobs.has(path, config.Key), "blocked: not planned")

	s.isCollectorJoinable = true
	s.scheduleEntries(path, []schedule.Config{config})
	require.True(t, s.jobs.has(path, config.Key), "unblocked: planned")
	j, _ := s.jobs.get(path, config.Key)
	j.entry.LastRunAt = s.clock.Add(-time.Minute)

	s.isCollectorJoinable = false
	s.scheduleEntries(path, []schedule.Config{config})
	assert.False(t, s.jobs.has(path, config.Key), "blocked again: unplanned")

	s.isCollectorJoinable = true
	s.scheduleEntries(path, []schedule.Config{config})
	require.True(t, s.jobs.has(path, config.Key))

	j, _ = s.jobs.get(path, config.Key)
	last := s.clock.Add(-time.Minute)
	j.entry.LastRunAt = last
	config.Schedule = "@1h"
	s.scheduleEntries(path, []schedule.Config{config})
	j, _ = s.jobs.get(path, config.Key)
	assert.Equal(t, last, j.entry.LastRunAt, "a schedule change keeps the last run")
	assert.Equal(t, last.Add(time.Hour), j.entry.NextRunAt, "and counts the new schedule from it")

	config.Schedule = "@0"
	s.scheduleEntries(path, []schedule.Config{config})
	assert.False(t, s.jobs.has(path, config.Key), "@0: unplanned")

	config.Schedule = "@10m"
	s.scheduleEntries(path, []schedule.Config{config})
	require.True(t, s.jobs.has(path, config.Key))
	s.scheduleEntries(path, nil)
	assert.False(t, s.jobs.has(path, config.Key), "no longer configured: unplanned")
	_, ok := s.schedules.Get(path, config.Key)
	assert.False(t, ok, "and its schedule forgotten")
}

// The queue gives the job due the soonest, whatever the order the jobs were
// planned and unplanned in.
func TestQueueOrder(t *testing.T) {
	q := newJobs()
	base := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	offsets := rand.Perm(200)
	for i, offset := range offsets {
		e := schedule.Entry{Config: schedule.Config{Key: time.Duration(i).String()}}
		e.NextRunAt = base.Add(time.Duration(offset) * time.Second)
		q.set(e, base)
	}
	// Unplan some, move some.
	for i := 0; i < 200; i += 3 {
		q.del(naming.Path{}, time.Duration(i).String())
	}
	for i := 1; i < 200; i += 7 {
		j, ok := q.get(naming.Path{}, time.Duration(i).String())
		if !ok {
			continue
		}
		e := j.entry
		e.NextRunAt = base.Add(-time.Duration(i) * time.Second)
		q.set(e, base)
	}
	var prev time.Time
	n := 0
	for _, j := range q.popDue(base.Add(time.Hour)) {
		assert.False(t, j.entry.NextRunAt.Before(prev), "out of order")
		prev = j.entry.NextRunAt
		n++
	}
	assert.Equal(t, q.len(), n, "every job was due")
	assert.Nil(t, q.first())
}

// The maps by object forget an object at once, which they did not: the key
// of an object was never found to delete, so a deleted object left its last
// runs and its requirements behind for the next object of the same name.
func TestPathKeyMapUnsetPath(t *testing.T) {
	m := make(timeMap)
	other := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "other"}
	m.Set(testPath, "a", time.Now())
	m.Set(testPath, "b", time.Now())
	m.Set(other, "a", time.Now())
	m.UnsetPath(testPath)
	_, ok := m.Get(testPath, "a")
	assert.False(t, ok)
	_, ok = m.Get(other, "a")
	assert.True(t, ok)
	m.Unset(other, "a")
	assert.Empty(t, m)
}
