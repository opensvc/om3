package ressync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestScheduleMaxDelay(t *testing.T) {
	last := time.Date(2026, 9, 28, 19, 0, 16, 0, time.Local)
	require.Equal(t, 45*time.Second, scheduleMaxDelay("@30s", last))
	require.Equal(t, 90*time.Minute, scheduleMaxDelay("@60m", last))
	daily := scheduleMaxDelay("02:00", last)
	require.Equal(t, 7*time.Hour-16*time.Second+12*time.Hour, daily, "due at 02:00, late at 14:00")
	require.Zero(t, scheduleMaxDelay("", last))
}

// A copy misses the writes made since its last sync, which end when the
// instance writing them stops: the copy of a stopped instance does not age.
func TestWritesMissed(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	lastSync := now.Add(-3 * time.Hour)
	require.Equal(t, 3*time.Hour, writesMissed(lastSync, time.Time{}, now), "writing: the time since the last sync")
	require.Equal(t, 10*time.Minute, writesMissed(lastSync, lastSync.Add(10*time.Minute), now), "stopped 10 minutes after the last sync")
	require.Equal(t, time.Duration(0), writesMissed(lastSync, lastSync.Add(-time.Minute), now), "synced after the writes stopped, as a final sync")
}
