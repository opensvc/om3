package tombstone

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rec(holders map[string]Holder) Record {
	return Record{
		OrchestrationID: "6e28098b-30bd-427c-941a-5b967353d0dd",
		Path:            "system/sec/s1",
		Kind:            KindDeleted,
		PurgedAt:        time.Date(2026, 11, 17, 15, 33, 47, 0, time.UTC),
		Holders:         holders,
	}
}

var (
	pendingLive = Holder{State: HolderPending, Mode: HolderLive}
	pendingAway = Holder{State: HolderPending, Mode: HolderAway}
	clearedDel  = Holder{State: HolderCleared, Mode: HolderLive, ClearedBy: ClearedByDeleted}
	clearedKept = Holder{State: HolderCleared, Mode: HolderAway, ClearedBy: ClearedByKept}
)

// Every node merging the copies it gets, in any order, ends with the same
// record: a cleared holder wins, of two cleared the higher rank, and away
// wins over live.
func TestMerge(t *testing.T) {
	copies := []Record{
		rec(map[string]Holder{"n2": pendingLive, "n3": pendingLive}),
		rec(map[string]Holder{"n2": clearedDel, "n3": pendingAway}),
		rec(map[string]Holder{"n3": clearedKept}),
	}
	want := rec(map[string]Holder{
		"n2": clearedDel,
		"n3": {State: HolderCleared, Mode: HolderAway, ClearedBy: ClearedByKept},
	})
	for _, order := range [][]int{{0, 1, 2}, {2, 1, 0}, {1, 0, 2}, {2, 0, 1}} {
		m := copies[order[0]].DeepCopy()
		for _, i := range order[1:] {
			m, _ = Merge(m, copies[i])
		}
		assert.True(t, want.Equal(m), "order %v: %+v", order, m.Holders)
	}
	m, changed := Merge(want, want)
	assert.False(t, changed, "idempotent")
	assert.True(t, want.Equal(m))
	_, changed = Merge(copies[0], copies[1])
	assert.True(t, changed)
}

func TestRecordPending(t *testing.T) {
	r := rec(map[string]Holder{"n2": pendingLive, "n3": pendingAway, "n4": clearedDel})
	assert.True(t, r.HasPending())
	assert.Equal(t, []string{"n2", "n3"}, r.PendingHolders(""))
	assert.Equal(t, []string{"n2"}, r.PendingHolders(HolderLive))
	assert.Equal(t, []string{"n3"}, r.PendingHolders(HolderAway))
	assert.True(t, r.IsPending("n3"))
	assert.False(t, r.IsPending("n4"))
	assert.False(t, rec(map[string]Holder{"n4": clearedDel}).HasPending())
}

// A record is saved atomically and loaded back; a corrupt file is skipped,
// its error returned, the others loaded.
func TestStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tombstone")
	l, err := Load(dir)
	require.NoError(t, err, "an absent directory holds no record")
	require.Empty(t, l)

	r := rec(map[string]Holder{"n2": pendingAway})
	require.NoError(t, Save(dir, r))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{"), 0o600))
	l, err = Load(dir)
	require.Error(t, err)
	require.Len(t, l, 1)
	assert.True(t, r.Equal(l[0]))

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp", "no temporary file left")
	}

	require.NoError(t, Remove(dir, r.OrchestrationID))
	require.NoError(t, Remove(dir, r.OrchestrationID), "removing an absent record is no error")
	require.NoError(t, os.Remove(filepath.Join(dir, "corrupt.json")))
	l, err = Load(dir)
	require.NoError(t, err)
	require.Empty(t, l)

	require.Error(t, Save(dir, Record{OrchestrationID: "../x"}))
}

// The pending holders of a path are read from the copies of every node
// merged: a holder one node saw cleared is cleared.
func TestDataPendingHolders(t *testing.T) {
	d := NewData()
	d.Set("n1", rec(map[string]Holder{"n2": pendingLive, "n3": pendingAway}))
	d.Set("n2", rec(map[string]Holder{"n2": clearedDel, "n3": pendingAway}))
	assert.Equal(t, []string{"n3"}, d.PendingHolders("system/sec/s1", ""))
	assert.Empty(t, d.PendingHolders("system/sec/s1", HolderLive))
	assert.True(t, d.IsPending("system/sec/s1", "n3"))
	assert.False(t, d.IsPending("system/sec/s1", "n2"))
	assert.False(t, d.IsPending("system/sec/other", "n3"))
	assert.Equal(t, []string{"n1", "n2"}, d.Publishers(rec(nil).OrchestrationID))

	got := d.GetByNode("n1")
	got[rec(nil).OrchestrationID].Holders["n2"] = clearedKept
	r, _ := d.Get("n1", rec(nil).OrchestrationID)
	assert.Equal(t, pendingLive, r.Holders["n2"], "the copies returned share nothing")

	d.Unset("n1", rec(nil).OrchestrationID)
	d.Unset("n2", rec(nil).OrchestrationID)
	assert.Empty(t, d.PendingHolders("system/sec/s1", ""))
}
