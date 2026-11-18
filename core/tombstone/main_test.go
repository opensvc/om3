package tombstone

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var version = time.Date(2026, 11, 17, 15, 33, 47, 0, time.UTC)

func rec(id string) Record {
	return Record{OrchestrationID: id, Path: "system/sec/s1", Kind: KindDeleted, ObjectID: "o1", Version: version}
}

// A record kills the version it deleted and the older ones, of its path
// only; a newer version, kept, created again or restored, is another.
func TestKills(t *testing.T) {
	r := rec("a")
	assert.True(t, r.Kills("system/sec/s1", version))
	assert.True(t, r.Kills("system/sec/s1", version.Add(-time.Hour)), "an older version, as one a node away missed an edit of")
	assert.False(t, r.Kills("system/sec/s1", version.Add(time.Nanosecond)), "a newer version")
	assert.False(t, r.Kills("system/sec/s2", version), "another path")
	assert.False(t, r.Kills("system/sec/s1", time.Time{}), "no known time")
	assert.False(t, Record{Path: "system/sec/s1"}.Kills("system/sec/s1", version), "no version recorded")
}

// A record is written and loaded back; a corrupt file is skipped, its error
// returned, the others loaded.
func TestStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tombstone")
	l, err := Load(dir)
	require.NoError(t, err, "an absent directory holds no record")
	require.Empty(t, l)

	r := rec("6e28098b-30bd-427c-941a-5b967353d0dd")
	require.NoError(t, Write(dir, r))
	require.NoError(t, SyncDir(dir))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{"), 0o600))
	l, err = Load(dir)
	require.Error(t, err)
	require.Len(t, l, 1)
	assert.Equal(t, r.OrchestrationID, l[0].OrchestrationID)
	assert.True(t, r.Version.Equal(l[0].Version))

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".tmp", "no temporary file left")
	}
	require.NoError(t, RemoveFile(dir, r.OrchestrationID))
	require.NoError(t, RemoveFile(dir, r.OrchestrationID), "removing an absent record is no error")
	require.NoError(t, os.Remove(filepath.Join(dir, "corrupt.json")))
	l, err = Load(dir)
	require.NoError(t, err)
	require.Empty(t, l)
	require.Error(t, Write(dir, Record{OrchestrationID: "../x"}))
}

// The records killing a config are read from every node, a trusted one
// first; a record keeps the trust it was first held with.
func TestKilledBy(t *testing.T) {
	d := NewData()
	d.Set("n1", rec("a"), false)
	d.Set("n2", rec("b"), true)
	l := d.KilledBy("system/sec/s1", version)
	require.Len(t, l, 2)
	assert.Equal(t, "n2", l[0].Node)
	assert.True(t, l[0].Trusted)
	assert.False(t, l[1].Trusted)
	assert.Empty(t, d.KilledBy("system/sec/s1", version.Add(time.Second)))
	assert.True(t, d.Kills("system/sec/s1", version))

	d.Set("n2", rec("b"), false)
	assert.True(t, d.KilledBy("system/sec/s1", version)[0].Trusted, "still trusted")
	d.Set("n1", rec("a"), true)
	assert.False(t, d.KilledBy("system/sec/s1", version)[1].Trusted, "heard again, as republished after a restart: still untrusted")

	d.Unset("n2", "b")
	d.Unset("n1", "a")
	assert.False(t, d.Kills("system/sec/s1", version))
	assert.Empty(t, d.All())
}

// A copy of a record shares nothing with it, nor does the holder with the
// records it is given or gives.
func TestDeepCopy(t *testing.T) {
	rec := Record{OrchestrationID: "a", Nodes: []string{"n1", "n2"}}
	c := rec.DeepCopy()
	c.Nodes[0] = "x"
	assert.Equal(t, "n1", rec.Nodes[0])

	d := NewData()
	d.Set("n1", rec, true)
	rec.Nodes[0] = "y"
	got, _ := d.Get("n1", "a")
	assert.Equal(t, "n1", got.Nodes[0])
	got.Nodes[1] = "z"
	again, _ := d.Get("n1", "a")
	assert.Equal(t, "n2", again.Nodes[1])
}
