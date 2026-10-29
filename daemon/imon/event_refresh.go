package imon

import (
	"time"
)

const (
	// eventRefreshDelay is how long after the first event asking for it the
	// local instance status is refreshed. The monitors report one change of
	// the node from different sources at nearly the same time, as a drbd
	// promotion and the mount that follows it, and the events arriving
	// meanwhile join the refresh already scheduled.
	eventRefreshDelay = time.Second

	// eventRefreshInterval is the least time between the last evaluation of
	// the local instance status, whatever asked for it, and a refresh asked
	// by an event, so a flow of events, or the tail of the events of an
	// action that just refreshed the status, does not have the daemon
	// evaluate it again and again.
	eventRefreshInterval = 5 * time.Second
)

// eventRefreshDelayAfter is how long from now to refresh a status an event
// asks to refresh, the last status evaluation being at last.
func eventRefreshDelayAfter(now, last time.Time) time.Duration {
	due := now.Add(eventRefreshDelay)
	if floor := last.Add(eventRefreshInterval); due.Before(floor) {
		due = floor
	}
	return due.Sub(now)
}

// scheduleEventRefresh schedules a refresh of the local instance status for
// an event it depends on: a mount, an address or a drbd state changed on the
// node, a peer instance changed, a peer wrote a state file. It is the one
// path of the event-driven refreshes, which coalesces them and paces them.
// A refresh already scheduled takes the event in, and is not pushed later,
// so a steady flow of events still gets its refreshes.
func (t *Manager) scheduleEventRefresh(reason string) {
	if t.eventRefreshScheduled {
		t.log.Tracef("status refresh already scheduled, joined by: %s", reason)
		return
	}
	delay := eventRefreshDelayAfter(time.Now(), t.instStatus[t.localhost].UpdatedAt)
	t.eventRefreshScheduled = true
	t.eventRefreshTimer.Reset(delay)
	t.log.Debugf("refresh the status in %s: %s", delay.Round(time.Millisecond), reason)
}

// onEventRefreshTimer refreshes the local instance status for the events
// that asked for it. With an action running on an instance, the refresh is
// deferred until none runs one: a local action refreshes the status when
// done, but the end of an action on a peer does not, and a refresh dropped,
// as the one a volume up asked while the peers still provisioned, left the
// status stale until the next scheduled status.
func (t *Manager) onEventRefreshTimer() {
	t.eventRefreshScheduled = false
	if !t.canRefreshOnEvent() {
		t.log.Tracef("defer the refresh asked by events: an action runs")
		t.eventRefreshDeferred = true
		return
	}
	t.eventRefreshDeferred = false
	t.requestStatusRefresh(t.instConfig.Priority)
}

// replayDeferredEventRefresh schedules the refresh deferred while an action
// ran on an instance, once none runs one. It is called on a change of the
// state of the local or a peer instance.
func (t *Manager) replayDeferredEventRefresh() {
	if !t.eventRefreshDeferred || !t.canRefreshOnEvent() {
		return
	}
	t.eventRefreshDeferred = false
	t.scheduleEventRefresh("deferred while an action ran")
}
