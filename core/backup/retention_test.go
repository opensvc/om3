package backup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRetention(t *testing.T) {
	r, err := ParseRetention("10 24h 7d 4w 6m 1y")
	require.NoError(t, err)
	assert.Equal(t, 10, r.Last)
	assert.Equal(t, []Period{{Hour, 24}, {Day, 7}, {Week, 4}, {Month, 6}, {Year, 1}}, r.Periods)
	assert.Equal(t, "10 24h 7d 4w 6m 1y", r.String())

	r, err = ParseRetention("3d 5d 2")
	require.NoError(t, err)
	assert.Equal(t, "2 5d", r.String(), "a unit given twice keeps the larger count")

	r, err = ParseRetention("")
	require.NoError(t, err)
	assert.True(t, r.IsZero(), "no keep keeps every run")

	for _, s := range []string{"0", "0d", "7x", "d", "-1", "2/d", "7 days"} {
		_, err := ParseRetention(s)
		assert.Error(t, err, s)
	}
}

func runsAt(times ...string) []Run {
	l := make([]Run, len(times))
	for i, s := range times {
		tm, err := time.Parse(time.RFC3339, s)
		if err != nil {
			panic(err)
		}
		l[i] = Run{ID: s, CreatedAt: tm}
	}
	return l
}

func ids(runs []Run) []string {
	l := make([]string, len(runs))
	for i, run := range runs {
		l[i] = run.ID
	}
	return l
}

func TestRetentionDrop(t *testing.T) {
	runs := runsAt(
		"2026-10-01T02:00:00Z",
		"2026-10-02T02:00:00Z",
		"2026-10-02T14:00:00Z",
		"2026-10-03T02:00:00Z",
		"2026-10-03T14:00:00Z",
	)

	r, _ := ParseRetention("2")
	assert.Equal(t, []string{"2026-10-01T02:00:00Z", "2026-10-02T02:00:00Z", "2026-10-02T14:00:00Z"}, ids(r.Drop(runs)), "the latest runs")

	r, _ = ParseRetention("2d")
	assert.Equal(t, []string{"2026-10-01T02:00:00Z", "2026-10-02T02:00:00Z", "2026-10-03T02:00:00Z"}, ids(r.Drop(runs)), "the latest run of each of the latest days")

	r, _ = ParseRetention("1 2d")
	assert.Equal(t, []string{"2026-10-01T02:00:00Z", "2026-10-02T02:00:00Z", "2026-10-03T02:00:00Z"}, ids(r.Drop(runs)), "a run kept by any rule is kept")

	r, _ = ParseRetention("")
	assert.Empty(t, r.Drop(runs), "no keep drops nothing")
}

// The periods are the ones holding a run: runs that stopped for months keep
// their last ones.
func TestRetentionDropCountsPeriodsHoldingRuns(t *testing.T) {
	runs := runsAt(
		"2026-01-01T02:00:00Z",
		"2026-01-02T02:00:00Z",
		"2026-01-03T02:00:00Z",
		"2026-09-01T02:00:00Z",
	)
	r, _ := ParseRetention("3d")
	assert.Equal(t, []string{"2026-01-01T02:00:00Z"}, ids(r.Drop(runs)))
}

// A day is a day in UTC, whatever the time zone a run is stamped in, so the
// runs of nodes in different zones fall in the same days.
func TestRetentionDropInUTC(t *testing.T) {
	paris := time.FixedZone("CEST", 2*3600)
	runs := []Run{
		{ID: "a", CreatedAt: time.Date(2026, 10, 2, 1, 0, 0, 0, paris)}, // 2026-10-01T23:00Z
		{ID: "b", CreatedAt: time.Date(2026, 10, 1, 22, 0, 0, 0, time.UTC)},
		{ID: "c", CreatedAt: time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)},
	}
	r, _ := ParseRetention("2d")
	assert.Equal(t, []string{"b"}, ids(r.Drop(runs)))
}

func TestRetentionDropWeeks(t *testing.T) {
	runs := runsAt(
		"2026-10-05T02:00:00Z", // monday, week 41
		"2026-10-11T02:00:00Z", // sunday, week 41
		"2026-10-12T02:00:00Z", // monday, week 42
	)
	r, _ := ParseRetention("2w")
	assert.Equal(t, []string{"2026-10-05T02:00:00Z"}, ids(r.Drop(runs)))
}

func TestFilterAndRun(t *testing.T) {
	run := Run{ID: "7c0e41d2-aaaa", Path: "ns1/svc/svc1", ObjectID: "oid", ClusterID: "c1", CompletedAt: time.Now(), Members: []Member{{Name: "fs#1", Kind: MemberDir}, {Name: MemberConfig, Kind: MemberStream}}}
	assert.True(t, RunFilter{}.Match(run))
	assert.True(t, RunFilter{ObjectID: "oid", Cluster: "c1", ID: "7c0e"}.Match(run))
	assert.False(t, RunFilter{ID: "7c0f"}.Match(run))
	assert.False(t, RunFilter{ObjectID: "oid2"}.Match(run))
	assert.False(t, RunFilter{Cluster: "c2"}.Match(run))
	assert.True(t, run.Has("fs#1"))
	assert.False(t, run.Has("fs#2"))
	assert.Equal(t, []string{"fs#1", "config"}, run.MemberNames())
	assert.Equal(t, "7c0e41d2", run.ShortID())
}

func TestExcluded(t *testing.T) {
	assert.True(t, Excluded("tmp", []string{"/tmp"}))
	assert.True(t, Excluded("a/b/c.log", []string{"*.log"}))
	assert.True(t, Excluded("a/b", []string{"a/b"}))
	assert.False(t, Excluded("x/a/b", []string{"a/b"}), "a pattern with a slash is anchored at the head")
	assert.False(t, Excluded("a", nil))
}

func TestHeadExcludes(t *testing.T) {
	excludes := []string{"fs#1:/tmp", "volume#1:/cache", "*.log", "a:b"}
	assert.Equal(t, []string{"/tmp", "*.log", "a:b"}, HeadExcludes("fs#1", excludes))
	assert.Equal(t, []string{"/cache", "*.log", "a:b"}, HeadExcludes("volume#1", excludes))
	assert.Equal(t, []string{"*.log", "a:b"}, HeadExcludes("fs#2", excludes))
}
