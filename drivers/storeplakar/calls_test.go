package storeplakar

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/backup"
)

// TestPlakarCallsOfTypicalOperations pins how many times plakar runs for
// the operations a user waits for, each call deriving the key of the store
// again: the listings of one operation are one plakar ls.
func TestPlakarCallsOfTypicalOperations(t *testing.T) {
	ctx := context.Background()
	run := run1
	run.ClusterID = "5a0ae2ee-2f5b-4a0c-9d9a-0f4b3a3f4c11"
	old := run
	old.ID = "r0"
	old.CreatedAt = t0.Add(-time.Hour)
	listing := []byte(
		lsLine(t0.Add(4*time.Minute), "s5", run, "om3.complete,om3.completed=2026-10-27T10%3A04%3A00Z") +
			lsLine(t0.Add(3*time.Minute), "s4", run, "om3.member=refs.json,om3.kind=stream") +
			lsLine(t0.Add(2*time.Minute), "s3", run, "om3.member=config,om3.kind=stream") +
			lsLine(t0.Add(time.Minute), "s2", run, "om3.member=fs%232,om3.kind=dir") +
			lsLine(t0, "s1", run, "om3.member=fs%231,om3.kind=dir") +
			lsLine(t0.Add(-time.Hour), "s0", old, "om3.complete,om3.completed=2026-10-27T09%3A00%3A00Z"))
	cases := []struct {
		name    string
		missing bool
		want    int
		do      func(t *testing.T, e testEnv)
	}{
		{
			name: "the stats of the store",
			want: 1,
			do: func(t *testing.T, e testEnv) {
				_, err := e.store.ObjectStats(ctx)
				require.NoError(t, err)
			},
		},
		{
			name:    "the stats of a store not created",
			missing: true,
			want:    0,
			do: func(t *testing.T, e testEnv) {
				_, err := e.store.ObjectStats(ctx)
				require.NoError(t, err)
			},
		},
		{
			name: "the restore of the latest run, two heads, the configuration and the keys",
			want: 5,
			do: func(t *testing.T, e testEnv) {
				found, err := backup.FindRun(ctx, e.store, backup.RunFilter{ObjectID: run.ObjectID, Cluster: run.ClusterID})
				require.NoError(t, err)
				_, err = backup.ReadMember(ctx, e.store, found, backup.MemberConfig)
				require.NoError(t, err)
				require.NoError(t, e.store.Restore(ctx, found, "fs#1", e.dst))
				require.NoError(t, e.store.Restore(ctx, found, "fs#2", e.dst))
				_, err = backup.ReadMember(ctx, e.store, found, backup.MemberRefs)
				require.NoError(t, err)
			},
		},
		{
			name: "the restore aside of a snapshot named by the path of its object, two heads",
			want: 3,
			do: func(t *testing.T, e testEnv) {
				found, err := backup.FindSnapshot(ctx, e.store, run.Path, run.ClusterID)
				require.NoError(t, err)
				require.NoError(t, e.store.Restore(ctx, found, "fs#1", e.dst))
				require.NoError(t, e.store.Restore(ctx, found, "fs#2", e.dst))
			},
		},
		{
			name: "the check of a run",
			want: 2,
			do: func(t *testing.T, e testEnv) {
				require.NoError(t, e.store.Check(ctx, backup.RunFilter{Path: run.Path, ID: run.ID}))
			},
		},
		{
			name: "the prune of one run after an update, the maintenance included",
			want: 3,
			do: func(t *testing.T, e testEnv) {
				runs, err := e.store.Runs(ctx, backup.RunFilter{ObjectID: run.ObjectID, Cluster: run.ClusterID, Incomplete: true})
				require.NoError(t, err)
				require.Len(t, runs, 2)
				require.NoError(t, e.store.Delete(ctx, runs[:1]))
			},
		},
	}
	for _, tc := range cases {
		e := newTestEnv(t)
		if !tc.missing {
			e.createStore(t)
		}
		setFake(t, "reply", listing)
		tc.do(t, e)
		n := len(calls(t))
		t.Logf("%d plakar calls for %s", n, tc.name)
		assert.Equal(t, tc.want, n, tc.name)
	}
}
