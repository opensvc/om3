package storeplakar

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/backup"
)

const (
	clusterID1 = "5a0ae2ee-2f5b-4a0c-9d9a-0f4b3a3f4c11"
	clusterID2 = "9e1f2a30-7c44-4c57-8a0b-6d2c5e1b7f22"
)

// lsVerbs returns the verb of every call made, with its -tag selection for
// a listing.
func lsVerbs(t *testing.T, e testEnv) []string {
	t.Helper()
	l := make([]string, 0)
	for _, argv := range calls(t) {
		words := argv[len(e.argv("@store")):]
		verb := words[0]
		for i, w := range words {
			if verb == "ls" && w == "-tag" {
				verb += " " + words[i+1]
			}
		}
		l = append(l, verb)
	}
	return l
}

// TestListingIsReadOncePerDriver pins the listings that follow one another
// served from one plakar ls: each call costs a key derivation, a second.
func TestListingIsReadOncePerDriver(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	ctx := context.Background()
	run := run1
	run.Members = []backup.Member{{Name: "config", Kind: backup.MemberStream}, {Name: "fs#1", Kind: backup.MemberDir}}
	setFake(t, "reply.1", []byte(
		lsLine(t0.Add(2*time.Minute), "s3", run1, "om3.complete,om3.completed=2026-10-27T10%3A02%3A00Z")+
			lsLine(t0.Add(time.Minute), "s2", run1, "om3.member=config,om3.kind=stream")+
			lsLine(t0, "s1", run1, "om3.member=fs%231,om3.kind=dir")))
	_, err := e.store.Runs(ctx, backup.RunFilter{ID: "r1"})
	require.NoError(t, err)
	_, err = e.store.Runs(ctx, backup.RunFilter{Path: "test/svc/app1"})
	require.NoError(t, err)
	_, err = e.store.ObjectStats(ctx)
	require.NoError(t, err)
	_, err = e.store.Cat(ctx, run, "config")
	require.NoError(t, err)
	require.NoError(t, e.store.Restore(ctx, run, "fs#1", e.dst))
	require.NoError(t, e.store.Check(ctx, backup.RunFilter{ID: "r1"}))
	assert.Equal(t, []string{"ls", "cat", "restore", "check"}, lsVerbs(t, e))
	assert.Equal(t, e.argv("@store", "restore", "-to", e.dst, "s1"), calls(t)[2], "the snapshot of the member, from the listing")
}

// TestWritesOutdateTheListing pins the listing read again after each write
// of the driver, and after its Close.
func TestWritesOutdateTheListing(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	ctx := context.Background()
	list := func() {
		t.Helper()
		_, err := e.store.Runs(ctx, backup.RunFilter{})
		require.NoError(t, err)
	}
	list()
	list()
	require.NoError(t, e.store.Backup(ctx, run1, "fs#1", e.src, e.src, nil))
	list()
	list()
	require.NoError(t, e.store.BackupStream(ctx, run1, "config", strings.NewReader("x")))
	list()
	require.NoError(t, e.store.Complete(ctx, run1))
	list()
	require.NoError(t, e.store.Delete(ctx, []backup.Run{run1}))
	list()
	require.NoError(t, e.store.Close())
	list()
	assert.Equal(t, []string{"ls", "backup", "ls", "backup", "ls", "backup", "ls", "rm", "maintenance", "ls", "ls"}, lsVerbs(t, e))
}

// TestListingIsNarrowedToTheObjectOfACluster pins the listing of the runs
// of an object of a cluster named by its id asked to plakar with the two
// tags, which it both requires, and the whole listing otherwise: a cluster
// name is not a tag every snapshot holds as it is now.
func TestListingIsNarrowedToTheObjectOfACluster(t *testing.T) {
	ctx := context.Background()
	run := run1
	run.ClusterID = clusterID1
	narrowed := "ls om3.cluster=" + clusterID1 + ",om3.object=o1"
	cases := map[string]struct {
		filter backup.RunFilter
		want   []string
	}{
		"object and cluster id":   {backup.RunFilter{ObjectID: "o1", Cluster: clusterID1}, []string{narrowed}},
		"with a path and an id":   {backup.RunFilter{ObjectID: "o1", Cluster: clusterID1, Path: "test/svc/app1", ID: "r"}, []string{narrowed}},
		"object and cluster name": {backup.RunFilter{ObjectID: "o1", Cluster: "cluster1"}, []string{"ls"}},
		"object alone":            {backup.RunFilter{ObjectID: "o1"}, []string{"ls"}},
		"cluster alone":           {backup.RunFilter{Cluster: clusterID1}, []string{"ls"}},
		"none":                    {backup.RunFilter{}, []string{"ls"}},
	}
	for name, tc := range cases {
		e := newTestEnv(t)
		e.createStore(t)
		setFake(t, "reply", []byte(
			lsLine(t0.Add(time.Minute), "s2", run, "om3.complete,om3.completed=2026-10-27T10%3A01%3A00Z")+
				lsLine(t0, "s1", run, "om3.member=fs%231,om3.kind=dir")))
		runs, err := e.store.Runs(ctx, tc.filter)
		require.NoError(t, err, name)
		require.Len(t, runs, 1, name)
		assert.Equal(t, "r1", runs[0].ID, name)
		assert.Equal(t, tc.want, lsVerbs(t, e), name)
	}
}

// TestNarrowedListingServesTheMembersOfItsRuns pins the restore of the
// latest run of an object, found in a narrowed listing: its members are
// looked up there, with no other listing.
func TestNarrowedListingServesTheMembersOfItsRuns(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	ctx := context.Background()
	run := run1
	run.ClusterID = clusterID1
	setFake(t, "reply.1", []byte(
		lsLine(t0.Add(time.Minute), "s2", run, "om3.complete,om3.completed=2026-10-27T10%3A01%3A00Z")+
			lsLine(t0, "s1", run, "om3.member=fs%231,om3.kind=dir")))
	found, err := backup.FindRun(ctx, e.store, backup.RunFilter{ObjectID: "o1", Cluster: clusterID1})
	require.NoError(t, err)
	require.NoError(t, e.store.Restore(ctx, found, "fs#1", e.dst))
	assert.Equal(t, []string{"ls om3.cluster=" + clusterID1 + ",om3.object=o1", "restore"}, lsVerbs(t, e))

	// A run of another object is not in that listing: its snapshots
	// are listed by its id, once for its members.
	other := run1
	other.ID = "r7"
	other.ObjectID = "o7"
	other.ClusterID = clusterID2
	setFake(t, "reply.3", []byte(
		lsLine(t0, "s8", other, "om3.member=fs%232,om3.kind=dir")+
			lsLine(t0, "s7", other, "om3.member=fs%231,om3.kind=dir")))
	require.NoError(t, e.store.Restore(ctx, other, "fs#1", e.dst))
	require.NoError(t, e.store.Restore(ctx, other, "fs#2", e.dst))
	assert.Equal(t, []string{"ls om3.run=r7", "restore", "restore"}, lsVerbs(t, e)[2:])
	assert.Equal(t, e.argv("@store", "restore", "-to", e.dst, "s8"), calls(t)[4])
}

// TestEmptyNarrowedListingIsReadAgainWhole pins the whole listing read when
// the narrowed one is empty: a cluster may be named as an id is, and its
// name is not what the snapshots are tagged with.
func TestEmptyNarrowedListingIsReadAgainWhole(t *testing.T) {
	e := newTestEnv(t)
	e.createStore(t)
	run := run1
	run.ClusterName = clusterID2
	setFake(t, "reply.2", []byte(lsLine(t0, "s1", run, "om3.complete,om3.completed=2026-10-27T10%3A00%3A00Z")))
	runs, err := e.store.Runs(context.Background(), backup.RunFilter{ObjectID: "o1", Cluster: clusterID2})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, []string{"ls om3.cluster=" + clusterID2 + ",om3.object=o1", "ls"}, lsVerbs(t, e))
}
