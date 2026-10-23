package backup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The runs of an object moved from a node to another are in the directories
// of both, and merge into one object.
func TestMergeObjectStats(t *testing.T) {
	t0 := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	n1 := []ObjectStats{{ClusterID: "c1", ObjectID: "o1", Path: "old", Runs: 2, Nodes: []string{"n1"}, LastCreatedAt: t0, LogicalBytes: 10}}
	n2 := []ObjectStats{
		{ClusterID: "c1", ObjectID: "o1", Path: "new", Runs: 1, Incomplete: 1, Nodes: []string{"n2"}, LastCreatedAt: t0.Add(time.Hour), LogicalBytes: 5},
		{ClusterID: "c1", ObjectID: "o2", Path: "other", Runs: 1, Nodes: []string{"n2"}},
	}
	l := MergeObjectStats(n1, n2)
	require.Len(t, l, 2)
	assert.Equal(t, 3, l[0].Runs)
	assert.Equal(t, 1, l[0].Incomplete)
	assert.Equal(t, []string{"n1", "n2"}, l[0].Nodes)
	assert.Equal(t, "new", l[0].Path, "the path of the latest run")
	assert.Equal(t, int64(15), l[0].LogicalBytes)

	s := SumStats(l)
	assert.Equal(t, 4, s.Runs)
	assert.Equal(t, 2, s.Objects)
	assert.Nil(t, s.StoredBytes, "unknown when an object with data does not tell it")
}

// An object counts once it holds a run, complete or not, and the cluster
// breakdown sums to the totals.
func TestNewObjectStatsAndSum(t *testing.T) {
	t0 := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	o := NewObjectStats("c1", "cl1", "o1", "svc1", []Run{
		{Node: "n2", CreatedAt: t0, CompletedAt: t0.Add(time.Minute), LogicalBytes: 7, StoredBytes: 3},
		{Node: "n1", CreatedAt: t0.Add(time.Hour), CompletedAt: t0.Add(time.Hour + time.Minute), Protected: true, LogicalBytes: 7, StoredBytes: 3},
		{Node: "n1", CreatedAt: t0.Add(2 * time.Hour)},
	})
	assert.Equal(t, 2, o.Runs)
	assert.Equal(t, 1, o.Incomplete)
	assert.Equal(t, 1, o.Protected)
	assert.Equal(t, []string{"n1", "n2"}, o.Nodes)
	assert.Equal(t, t0, o.FirstCreatedAt)
	assert.Equal(t, t0.Add(time.Hour), o.LastCreatedAt, "of the completed runs")

	empty := NewObjectStats("c1", "cl1", "o2", "svc2", nil)
	s := SumStats([]ObjectStats{o, empty})
	assert.Equal(t, 1, s.Objects, "an object without a run is not counted")
	require.NotNil(t, s.StoredBytes)
	assert.Equal(t, int64(6), *s.StoredBytes)
	require.Len(t, s.Clusters, 1)
	assert.Equal(t, s.Runs, s.Clusters[0].Runs)
}

// A filter by time selects the runs of the period, and tells the objects
// whose runs are all out of it.
func TestTimeFilter(t *testing.T) {
	at := []time.Time{
		time.Date(2026, 10, 19, 22, 30, 0, 0, time.UTC),
		time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC),
	}
	f := RunFilter{Since: at[1], Until: at[2]}
	done := func(t time.Time) Run { return Run{CreatedAt: t, CompletedAt: t} }
	assert.False(t, f.Match(done(at[0])))
	assert.True(t, f.Match(done(at[1])), "since is inclusive")
	assert.False(t, f.Match(done(at[2])), "until is exclusive")
	assert.False(t, f.MayHold(at[2], at[2]), "an object with runs after the period only")
	assert.True(t, f.MayHold(at[0], at[2]), "an object with runs around the period")
}
