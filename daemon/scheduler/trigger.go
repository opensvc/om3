package scheduler

import (
	"slices"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	// trigger paces the runs the events of an entry ask for: a run at the
	// first event, then one at most each interval, the events coming
	// during an interval running the entry once at its end.
	//
	// An interval ending with such a run doubles the next one, up to the
	// backoff max, and each interval passing without an event halves it
	// back.
	trigger struct {
		spec     string
		pace     schedule.OnPace
		interval time.Duration

		// on are the events of the entry the trigger was made for: a run
		// due for one of them is not one of other events.
		on []string

		// lastRunAt is when the entry last ran for an event.
		lastRunAt time.Time

		// dueAt is when the entry runs for the events received since its
		// last run, zero when none is.
		dueAt time.Time
	}

	triggerMap = pathKeyMap[*trigger]
)

func newTrigger(spec string, on []string) (*trigger, error) {
	pace, err := schedule.ParsePace(spec)
	if err != nil {
		return nil, err
	}
	return &trigger{spec: spec, on: slices.Clone(on), pace: pace, interval: pace.Interval}, nil
}

// isFor says if the trigger was made for the pace and the events of e, and
// holds no run due for events e no longer runs on.
func (g *trigger) isFor(e schedule.Entry) bool {
	return g.spec == e.Pace && slices.Equal(g.on, e.On)
}

// onEvent says if an event received at now runs the entry now. When it does
// not, the entry runs at dueAt.
func (g *trigger) onEvent(now time.Time) bool {
	if !g.dueAt.IsZero() {
		return false
	}
	if g.lastRunAt.IsZero() {
		return true
	}
	next := g.lastRunAt.Add(g.interval)
	if now.Before(next) {
		g.dueAt = next
		return false
	}
	// The intervals passed without an event ease the backoff.
	for quiet := now.Sub(next); g.interval > g.pace.Interval && quiet >= g.interval; {
		quiet -= g.interval
		g.interval = max(g.interval/2, g.pace.Interval)
	}
	return true
}

// ran records the run of the entry at now, the end of an interval with
// events when late is true, which backs the next interval off.
func (g *trigger) ran(now time.Time, late bool) {
	g.lastRunAt = now
	g.dueAt = time.Time{}
	if late {
		g.interval = min(g.interval*2, g.pace.Max)
	}
}

func (t *T) onNetworkPeersUpdated(c *msgbus.NetworkPeersUpdated) {
	t.onEvent(c, naming.Path{})
}

// onFileInstalled runs the entries of the instance whose resource installed
// files, the files of an object being what its tasks read.
func (t *T) onFileInstalled(c *msgbus.FileInstalled) {
	t.onEvent(c, c.Path)
}

// onEvent runs the entries of the local instances whose on events match ev,
// of the instance of path only when path is not zero.
func (t *T) onEvent(ev pubsub.Messager, only naming.Path) {
	if !t.enabled {
		return
	}
	key := ev.(interface{ Key() string }).Key()
	now := t.now()
	for path, m := range t.schedules {
		if !only.IsZero() && path != only {
			continue
		}
		for _, e := range m {
			if !matchOn(e, key) {
				continue
			}
			g, err := t.getTrigger(e)
			if err != nil {
				t.jobLogger(e).Warnf("%s: %s", key, err)
				continue
			}
			logger := t.jobLogger(e)
			if !g.onEvent(now) {
				logger.Infof("on %s: due at %s", key, g.dueAt.Format(time.RFC3339))
				continue
			}
			logger.Infof("on %s: due now", key)
			t.runTrigger(path, e, g, now, false)
		}
	}
}

// matchOn says if the event whose key is key is one of the on events of e.
func matchOn(e schedule.Entry, key string) bool {
	for _, s := range e.On {
		f, err := schedule.ParseOn(s)
		if err != nil {
			continue
		}
		if f.Match(key) {
			return true
		}
	}
	return false
}

// getTrigger returns the trigger of e, new when its pace or its events
// changed.
func (t *T) getTrigger(e schedule.Entry) (*trigger, error) {
	if g, ok := t.triggers.Get(e.Path, e.Key); ok && g.isFor(e) {
		return g, nil
	}
	g, err := newTrigger(e.Pace, e.On)
	if err != nil {
		return nil, err
	}
	// The pace counts from the last run of the entry, whatever ran it, as
	// a daemon restarted reads it from the last run file: a trigger made
	// new does not run the entry again sooner than its pace allows.
	if g.lastRunAt = e.LastRunAt; g.lastRunAt.IsZero() {
		g.lastRunAt = e.GetLastRun()
	}
	t.triggers.Set(e.Path, e.Key, g)
	return g, nil
}

// runTrigger runs the entry e for its events, unless it can not run: a run
// held back by an action of the instance, or by a run of the entry, is tried
// again in a while, and a run the instance does not allow is dropped.
func (t *T) runTrigger(path naming.Path, e schedule.Entry, g *trigger, now time.Time, late bool) {
	logger := t.jobLogger(e)
	if reason := t.blockReason(e); reason != "" {
		logger.Infof("skip event run (%s)", reason)
		g.ran(now, late)
		return
	}
	if reason := t.holdReason(e); reason != "" {
		logger.Infof("hold event run (%s)", reason)
		g.dueAt = now.Add(holdRetryDelay)
		return
	}
	if n, err := t.runningCount(e); err != nil {
		logger.Warnf("skip event run (%s)", err)
		g.ran(now, late)
		return
	} else if n >= e.MaxParallel {
		logger.Infof("hold event run (%d/%d jobs already running)", n, e.MaxParallel)
		g.dueAt = now.Add(g.interval)
		return
	}
	g.ran(now, late)
	t.recordEventRun(path, e, now)
	t.run(e, now, logger)
}

// recordEventRun records a run of the entry e the events asked for as a run
// of e, as a scheduled run is: it is its last run, which the listings show
// and the peers hear of, and the scheduled run is planned from it, the work
// it would do being done.
func (t *T) recordEventRun(path naming.Path, e schedule.Entry, now time.Time) {
	e.LastRunAt = now
	t.schedules.Add(path, e)
	if j, ok := t.jobs.get(path, e.Key); ok {
		next := j.entry
		next.LastRunAt = now
		t.planJob(next, now)
	}
	t.lastRunOnAllPeers.Set(path, e.Key, now)
	t.updateExposedSchedules(path)
}

// onTriggersDue runs the entries whose events are due.
func (t *T) onTriggersDue(now time.Time) {
	for path, m := range t.triggers {
		for key, g := range m {
			if g.dueAt.IsZero() || g.dueAt.After(now) {
				continue
			}
			e, ok := t.schedules.Get(path, key)
			if !ok || len(e.On) == 0 {
				t.triggers.Unset(path, key)
				continue
			}
			t.jobLogger(e).Infof("on events: due now")
			t.runTrigger(path, e, g, now, true)
		}
	}
}

// nextTriggerAt returns when the soonest entry is due for its events, and
// false when none is.
func (t *T) nextTriggerAt() (time.Time, bool) {
	var next time.Time
	for _, m := range t.triggers {
		for _, g := range m {
			if g.dueAt.IsZero() {
				continue
			}
			if next.IsZero() || g.dueAt.Before(next) {
				next = g.dueAt
			}
		}
	}
	return next, !next.IsZero()
}
