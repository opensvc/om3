package imon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/util/plog"
)

// An event refresh runs a second after the event, and no sooner than the
// pacing interval after the last status evaluation.
func TestEventRefreshDelayAfter(t *testing.T) {
	now := time.Now()
	require.Equal(t, eventRefreshDelay, eventRefreshDelayAfter(now, time.Time{}), "never evaluated")
	require.Equal(t, eventRefreshDelay, eventRefreshDelayAfter(now, now.Add(-time.Minute)), "evaluated long ago")
	require.Equal(t, eventRefreshInterval-time.Second, eventRefreshDelayAfter(now, now.Add(-time.Second)), "evaluated a second ago")
}

// Events arriving while a refresh is scheduled join it: the mount and the
// drbd events of one change cause one refresh.
func TestScheduleEventRefreshCoalesces(t *testing.T) {
	m := &Manager{
		localhost:         "n1",
		log:               plog.NewDefaultLogger(),
		instStatus:        map[string]instance.Status{"n1": {}},
		eventRefreshTimer: time.NewTimer(time.Hour),
	}
	m.eventRefreshTimer.Stop()

	m.scheduleEventRefresh("drbd resource r1 changed")
	m.scheduleEventRefresh("mount point /srv/r1 mounted")
	m.scheduleEventRefresh("the availability changed on n2")

	select {
	case <-m.eventRefreshTimer.C:
	case <-time.After(eventRefreshDelay + time.Second):
		t.Fatal("the refresh did not fire")
	}
	select {
	case <-m.eventRefreshTimer.C:
		t.Fatal("the events caused more than one refresh")
	case <-time.After(eventRefreshDelay + 200*time.Millisecond):
	}
	require.True(t, m.eventRefreshScheduled, "the timer handler clears it, not run here")
}
