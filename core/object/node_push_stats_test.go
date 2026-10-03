package object

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// A range is pushed a local day at a time, each day ending before the
// midnight the next begins at, so a sample dated midnight is sent once.
func TestPushStatsBatches(t *testing.T) {
	at := func(day, hour, min int) time.Time { return time.Date(2026, 10, day, hour, min, 0, 0, time.Local) }
	midnight := func(day int) time.Time { return at(day, 0, 0) }

	l := pushStatsBatches(at(1, 18, 0), at(2, 10, 34))
	require.Len(t, l, 2)
	assert.Equal(t, [2]time.Time{at(1, 18, 0), midnight(2).Add(-time.Nanosecond)}, l[0])
	assert.Equal(t, [2]time.Time{midnight(2), at(2, 10, 34)}, l[1])

	l = pushStatsBatches(at(1, 1, 0), at(1, 2, 0))
	assert.Equal(t, [][2]time.Time{{at(1, 1, 0), at(1, 2, 0)}}, l, "a range within a day is one batch")

	l = pushStatsBatches(midnight(1), midnight(4))
	assert.Len(t, l, 3, "a range ending at midnight has no empty last batch")

	year := pushStatsBatches(time.Date(2025, 10, 1, 0, 0, 0, 0, time.Local), time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local))
	assert.Len(t, year, 365)
	for i := 1; i < len(year); i++ {
		assert.True(t, year[i][0].After(year[i-1][1]), "batches do not overlap")
	}
}

// The result reads as a person reads it, the stored counts left out of a dry
// run, which stores nothing.
func TestPushStatsResultRender(t *testing.T) {
	r := PushStatsResult{
		Begin:   time.Date(2026, 10, 1, 18, 0, 0, 0, time.Local),
		End:     time.Date(2026, 10, 2, 10, 34, 32, 0, time.Local),
		Batches: 2,
		Rows:    map[string]int{"cpu": 297, "block": 99},
		Series:  832,
		Points:  78448,
	}
	assert.Equal(t, `range     `+r.Begin.Format(time.RFC3339)+` to `+r.End.Format(time.RFC3339)+`
batches   2
rows      block 99, cpu 297
stored    832 series, 78448 points
`, r.Render())
	r.DryRun = true
	assert.Contains(t, r.Render(), "stored    nothing, this is a dry run")
}
