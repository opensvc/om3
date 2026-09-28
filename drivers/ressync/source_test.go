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
