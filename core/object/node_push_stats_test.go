package object

import (
	"testing"
	"time"
)

func TestPushStatsRange(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name       string
		options    PushStatsOptions
		last       time.Time
		begin, end time.Time
	}{
		{"never pushed", PushStatsOptions{}, time.Time{}, now.Add(-1450 * time.Minute), now},
		{"pushed an hour ago", PushStatsOptions{}, now.Add(-time.Hour), now.Add(-70 * time.Minute), now},
		{"pushed a minute ago", PushStatsOptions{}, now.Add(-time.Minute), now.Add(-21 * time.Minute), now},
		{"pushed a week ago", PushStatsOptions{}, now.Add(-7 * 24 * time.Hour), now.Add(-1450 * time.Minute), now},
		{
			"asked",
			PushStatsOptions{Begin: now.Add(-3 * 24 * time.Hour), End: now.Add(-time.Hour)},
			now.Add(-time.Hour),
			now.Add(-3 * 24 * time.Hour), now.Add(-time.Hour),
		},
		{"asked end", PushStatsOptions{End: now.Add(-time.Hour)}, time.Time{}, now.Add(-time.Hour - 1450*time.Minute), now.Add(-time.Hour)},
	} {
		begin, end := pushStatsRange(c.options, c.last, now)
		if !begin.Equal(c.begin) || !end.Equal(c.end) {
			t.Errorf("%s: got %s - %s, want %s - %s", c.name, begin, end, c.begin, c.end)
		}
	}
}
