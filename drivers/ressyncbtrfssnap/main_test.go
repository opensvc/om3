package ressyncbtrfssnap

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSnapName(t *testing.T) {
	named := &T{Name: "weekly"}
	unnamed := &T{}
	at := time.Date(2026, 10, 9, 10, 9, 52, 123456000, time.UTC)

	tm, ok := named.parseSnapName(named.snapName(at))
	require.True(t, ok)
	assert.Equal(t, at, tm)
	assert.Equal(t, "2026-10-09T10:09:52.123456Z,weekly", named.snapName(at))

	tm, ok = unnamed.parseSnapName("2016-03-09T10:09:52Z")
	require.True(t, ok, "v2 left out a zero fraction of second")
	assert.Equal(t, time.Date(2016, 3, 9, 10, 9, 52, 0, time.UTC), tm)

	_, ok = unnamed.parseSnapName(named.snapName(at))
	assert.False(t, ok, "the snapshots of a named resource are not of an unnamed one")
	_, ok = named.parseSnapName(unnamed.snapName(at))
	assert.False(t, ok)
	_, ok = named.parseSnapName("2026-10-09T10:09:52.123456Z,daily")
	assert.False(t, ok)
	_, ok = unnamed.parseSnapName("notadate")
	assert.False(t, ok)
}

// fakeBtrfs makes the snapshot and delete commands act on directories, the
// directories of the snapshots taking the inode number of a subvolume root,
// and the others the one of a placeholder.
func fakeBtrfs(t *testing.T) *[][]string {
	t.Helper()
	calls := &[][]string{}
	subvols := make(map[string]bool)
	prev, prevInodeOf := btrfsCmd, inodeOf
	t.Cleanup(func() { btrfsCmd, inodeOf = prev, prevInodeOf })
	btrfsCmd = func(args ...string) ([]byte, error) {
		*calls = append(*calls, args)
		switch {
		case len(args) == 5 && args[0] == "subvolume" && args[1] == "snapshot" && args[2] == "-r":
			subvols[args[4]] = true
			return nil, os.Mkdir(args[4], 0755)
		case len(args) == 3 && args[0] == "subvolume" && args[1] == "delete":
			if !subvols[args[2]] {
				return nil, fmt.Errorf("%s: not a subvolume", args[2])
			}
			delete(subvols, args[2])
			return nil, os.Remove(args[2])
		}
		return nil, fmt.Errorf("unexpected btrfs %v", args)
	}
	inodeOf = func(path string) (uint64, error) {
		if _, err := os.Lstat(path); err != nil {
			return 0, err
		}
		if subvols[path] {
			return subvolRootIno, nil
		}
		return placeholderIno, nil
	}
	return calls
}

func TestSnapAndPruneKeepsTheNewest(t *testing.T) {
	calls := fakeBtrfs(t)
	dir := t.TempDir()
	o := &T{Name: "daily", Keep: 2}
	other := &T{Name: "weekly"}
	t0 := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, snapDir, other.snapName(t0)), 0755))

	for i := 0; i < 4; i++ {
		require.NoError(t, o.snapAndPrune(dir, t0.Add(time.Duration(i)*time.Hour)))
	}
	l, _, err := o.snapshots(dir)
	require.NoError(t, err)
	require.Len(t, l, 2)
	assert.Equal(t, t0.Add(3*time.Hour), l[0].CreatedAt, "newest first")
	assert.Equal(t, t0.Add(2*time.Hour), l[1].CreatedAt)
	assert.DirExists(t, filepath.Join(dir, snapDir, other.snapName(t0)), "the snapshots of another resource are kept")
	assert.Len(t, *calls, 4+2)
}

func TestSnapshotsOfASubvolumeNeverSnapshotted(t *testing.T) {
	l, placeholders, err := (&T{}).snapshots(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, l)
	assert.Empty(t, placeholders)
}

func TestSnapAndPruneRemovesPlaceholders(t *testing.T) {
	calls := fakeBtrfs(t)
	dir := t.TempDir()
	o := &T{Keep: 2}
	t0 := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	var placeholders []string
	for i := 0; i < 3; i++ {
		p := filepath.Join(dir, snapDir, o.snapName(t0.Add(time.Duration(i)*time.Minute)))
		require.NoError(t, os.MkdirAll(p, 0755))
		placeholders = append(placeholders, p)
	}

	l, found, err := o.snapshots(dir)
	require.NoError(t, err)
	assert.Empty(t, l, "a placeholder is not a snapshot")
	assert.ElementsMatch(t, placeholders, found)

	require.NoError(t, o.snapAndPrune(dir, t0.Add(time.Hour)))
	for _, p := range placeholders {
		assert.NoDirExists(t, p)
	}
	l, found, err = o.snapshots(dir)
	require.NoError(t, err)
	require.Len(t, l, 1)
	assert.Equal(t, t0.Add(time.Hour), l[0].CreatedAt)
	assert.Empty(t, found)
	assert.Len(t, *calls, 1, "no subvolume delete of a placeholder")
}

// A name is part of the name of the snapshots, in the .snap directory: a '/'
// would have them taken out of it, out of the retention.
func TestCheckName(t *testing.T) {
	assert.NoError(t, (&T{}).checkName())
	assert.NoError(t, (&T{Name: "daily.1"}).checkName())
	assert.NoError(t, (&T{Name: ".."}).checkName(), "a part of the component, never one of its own")
	assert.Error(t, (&T{Name: "../../etc"}).checkName())
	assert.Error(t, (&T{Name: "a/b"}).checkName())
}

// Two btrfs have a subvolume data each: the one of the label is the one
// mounted from one of its devices.
func TestMountOfSubvolOfTheLabel(t *testing.T) {
	mi := []byte(`480 22 7:1 /data /srv/a rw,relatime shared:250 - btrfs /dev/loop1 rw,subvol=/data
481 22 7:2 /data /srv/b rw,relatime shared:251 - btrfs /dev/loop2 rw,subvol=/data
482 22 7:2 /data/x /srv/c rw,relatime shared:252 - btrfs /dev/loop2 rw,subvol=/data/x
`)
	mnt, ok := mountOfSubvol(mi, []string{"/dev/loop2"}, "data")
	require.True(t, ok)
	assert.Equal(t, "/srv/b", mnt)
	mnt, ok = mountOfSubvol(mi, []string{"/dev/loop3", "/dev/loop1"}, "/data/")
	require.True(t, ok)
	assert.Equal(t, "/srv/a", mnt, "a btrfs spanning several devices is mounted from any")
	_, ok = mountOfSubvol(mi, nil, "data")
	assert.False(t, ok, "no filesystem has the label on this node")
	_, ok = mountOfSubvol(mi, []string{"/dev/loop1"}, "data/x")
	assert.False(t, ok)
}
