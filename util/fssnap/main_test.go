package fssnap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeMounts(t *testing.T, mounts ...mountInfo) {
	t.Helper()
	prev := readMounts
	t.Cleanup(func() { readMounts = prev })
	readMounts = func() ([]mountInfo, error) { return mounts, nil }
}

var testMounts = []mountInfo{
	{Root: "/", Target: "/", FSType: "ext4", Source: "/dev/mapper/root-root"},
	{Root: "/", Target: "/srv", FSType: "xfs", Source: "/dev/mapper/data-srv"},
	{Root: "/data", Target: "/srv/b", FSType: "btrfs", Source: "/dev/loop1"},
	{Root: "/", Target: "/srv/b/tmp", FSType: "tmpfs", Source: "tmpfs"},
}

func TestHoldingMount(t *testing.T) {
	for p, want := range map[string]string{
		"/etc/hosts":   "/",
		"/srv":         "/srv",
		"/srv/a/b":     "/srv",
		"/srv/b":       "/srv/b",
		"/srv/bb":      "/srv",
		"/srv/b/x":     "/srv/b",
		"/srv/b/tmp/x": "/srv/b/tmp",
	} {
		m, ok := holdingMount(testMounts, p)
		require.True(t, ok, p)
		assert.Equal(t, want, m.Target, p)
	}
	over := append(append([]mountInfo{}, testMounts...), mountInfo{Root: "/", Target: "/srv", FSType: "ext4", Source: "/dev/sdz"})
	m, _ := holdingMount(over, "/srv/a")
	assert.Equal(t, "/dev/sdz", m.Source, "the last mount on a mount point hides the ones before it")
}

func TestMountBelow(t *testing.T) {
	holding, _ := holdingMount(testMounts, "/srv/b")
	below, ok := mountBelow(testMounts, "/srv/b", holding, "/var/lib/opensvc/x")
	require.True(t, ok)
	assert.Equal(t, "/srv/b/tmp", below.Target)

	holding, _ = holdingMount(testMounts, "/srv/b/x")
	_, ok = mountBelow(testMounts, "/srv/b/x", holding, "/var/lib/opensvc/x")
	assert.False(t, ok)

	snaps := append(append([]mountInfo{}, testMounts...),
		mountInfo{Root: "/", Target: "/var/lib/opensvc/x/mnt/0", FSType: "xfs", Source: "/dev/data/s"},
		mountInfo{Root: "/", Target: "/srv/z/.zfs/snapshot/s", FSType: "zfs", Source: "tank/z@s"},
	)
	holding, _ = holdingMount(snaps, "/")
	below, ok = mountBelow(snaps, "/var", holding, "/var/lib/opensvc/x")
	assert.False(t, ok, "the mounts of the set are not below a path: %v", below)
}

func TestCreateRefusesAMountBelow(t *testing.T) {
	dir := t.TempDir()
	fakeMounts(t, mountInfo{Root: "/", Target: "/", FSType: "ext4", Source: "/dev/sda1"},
		mountInfo{Root: "/", Target: filepath.Join(dir, "src", "sub"), FSType: "tmpfs", Source: "tmpfs"})
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src", "sub"), 0700))
	s := &Set{Dir: filepath.Join(dir, "set"), Name: "osvc_sync_t"}
	_, err := s.Create(context.Background(), []string{filepath.Join(dir, "src") + "/"})
	assert.ErrorContains(t, err, "set the copy to stay on one filesystem")
	assert.False(t, Pending(s.Dir), "nothing made")
}

func TestCheck(t *testing.T) {
	assert.Error(t, (&Set{Dir: "rel", Name: "n"}).check())
	assert.Error(t, (&Set{Dir: "/x", Name: "sync#1"}).check())
	assert.NoError(t, (&Set{Dir: "/x", Name: "osvc_sync_2a21fa45_sync.1"}).check())
	assert.Error(t, (&Set{Dir: "/x", Name: ".."}).check())
	assert.Error(t, (&Set{Dir: "/x", Name: "."}).check())
}

// A journal entry names what was about to be made, which a killed run may
// not have made: undoing it then has nothing to do, and the entry goes.
func TestRemoveUndoesTheJournal(t *testing.T) {
	dir := t.TempDir()
	fakeMounts(t, mountInfo{Root: "/", Target: "/", FSType: "ext4", Source: "/dev/sda1"})
	s := &Set{Dir: dir, Name: "n"}
	mnt := s.mountDir(0)
	require.NoError(t, s.makeMountDir(mnt))
	require.NoError(t, s.record(entry{Kind: kindMount, Path: mnt}))
	assert.True(t, Pending(dir))

	require.NoError(t, s.Remove(context.Background()))
	assert.False(t, Pending(dir))
	assert.NoDirExists(t, mnt)
	assert.NoFileExists(t, filepath.Join(dir, journalFile))
}

// What could not be undone stays in the journal, for the next Remove.
func TestRemoveKeepsWhatItCouldNotUndo(t *testing.T) {
	dir := t.TempDir()
	fakeMounts(t)
	s := &Set{Dir: dir, Name: "n"}
	require.NoError(t, s.record(entry{Kind: "unknown", Path: "x"}))
	require.NoError(t, s.record(entry{Kind: kindDir, Path: filepath.Join(dir, "gone")}))
	assert.Error(t, s.Remove(context.Background()))
	l, err := readJournal(dir)
	require.NoError(t, err)
	assert.Equal(t, []entry{{Kind: "unknown", Path: "x"}}, l)
}

func TestUndoMountRefusesAPathOutsideTheSet(t *testing.T) {
	s := &Set{Dir: "/var/lib/opensvc/x", Name: "n"}
	assert.ErrorContains(t, s.undoMount(context.Background(), "/srv"), "refuse to unmount")
	assert.ErrorContains(t, s.undoMount(context.Background(), "/var/lib/opensvc/x"), "refuse to unmount")
}

// A link named as the source is the link, which rsync copies as one, and the
// directory it points at when the path ends with a '/', which rsync follows.
func TestResolve(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "real", "sub"), 0755))
	require.NoError(t, os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "link")))
	require.NoError(t, os.Symlink(filepath.Join(dir, "nowhere"), filepath.Join(dir, "broken")))
	realDir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)

	p, err := resolve(filepath.Join(dir, "link"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(realDir, "link"), p, "the link itself")

	p, err = resolve(filepath.Join(dir, "link") + "/")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(realDir, "real"), p, "the directory it points at")

	p, err = resolve(filepath.Join(dir, "link", "sub"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(realDir, "real", "sub"), p, "the directories it is in resolved")

	p, err = resolve(filepath.Join(dir, "broken"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(realDir, "broken"), p, "a link pointing nowhere copies as such")
}
