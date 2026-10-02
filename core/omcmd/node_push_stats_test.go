package omcmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --begin and --end take a time ago, a date, or a time of today, in the local
// time zone when they name none.
func TestParsePushStatsTime(t *testing.T) {
	now := time.Date(2026, 10, 2, 10, 34, 32, 0, time.Local)
	for _, tc := range []struct {
		s    string
		want time.Time
	}{
		{"", time.Time{}},
		{"-1d2h", now.Add(-26 * time.Hour)},
		{"-1d", now.Add(-24 * time.Hour)},
		{"-30m", now.Add(-30 * time.Minute)},
		{"02:00", time.Date(2026, 10, 2, 2, 0, 0, 0, time.Local)},
		{"02:00:30", time.Date(2026, 10, 2, 2, 0, 30, 0, time.Local)},
		{"2025-10-01", time.Date(2025, 10, 1, 0, 0, 0, 0, time.Local)},
		{"2026-10-01 18:00", time.Date(2026, 10, 1, 18, 0, 0, 0, time.Local)},
		{"2026-10-01T18:00:00Z", time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC)},
	} {
		got, err := parsePushStatsTime("begin", tc.s, now)
		require.NoError(t, err, tc.s)
		assert.True(t, tc.want.Equal(got), "%q: got %s, want %s", tc.s, got, tc.want)
	}
	for _, s := range []string{"-", "-x", "1d", "25:00", "yesterday"} {
		_, err := parsePushStatsTime("begin", s, now)
		assert.Error(t, err, s)
	}
}
