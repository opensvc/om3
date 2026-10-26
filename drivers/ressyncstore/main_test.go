package ressyncstore

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func touch(t *testing.T, p string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
	require.NoError(t, os.WriteFile(p, nil, 0600))
}

// A head emptied before a restore in place keeps its excluded paths, at any
// depth, and loses everything else, symlinks not followed.
func TestEmptyDir(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	touch(t, filepath.Join(outside, "keep"))
	touch(t, filepath.Join(dir, "a"))
	touch(t, filepath.Join(dir, "sub", "b"))
	touch(t, filepath.Join(dir, "sub", "c.log"))
	touch(t, filepath.Join(dir, "tmp", "x"))
	touch(t, filepath.Join(dir, "data", "cache", "y"))
	touch(t, filepath.Join(dir, "data", "z"))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "link")))

	require.NoError(t, emptyDir(dir, []string{"/tmp", "*.log", "data/cache"}, nil))
	assert.NoFileExists(t, filepath.Join(dir, "a"))
	assert.NoFileExists(t, filepath.Join(dir, "sub", "b"))
	assert.FileExists(t, filepath.Join(dir, "sub", "c.log"))
	assert.FileExists(t, filepath.Join(dir, "tmp", "x"))
	assert.FileExists(t, filepath.Join(dir, "data", "cache", "y"))
	assert.NoFileExists(t, filepath.Join(dir, "data", "z"))
	assert.NoFileExists(t, filepath.Join(dir, "link"))
	assert.FileExists(t, filepath.Join(outside, "keep"), "a symlink is removed, not followed")

	require.NoError(t, emptyDir(dir, nil, nil))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// A member saved from a path restores under the deepest head holding it, and
// emptying it leaves the heads of the other resources under it.
func TestRestoreTargets(t *testing.T) {
	heads := []head{{rid: "fs#1", dir: "/srv/app"}, {rid: "fs#2", dir: "/srv/app/logs"}, {rid: "volume#1", dir: "/srv/vols/v1"}}
	h, ok := containingHead(heads, "/srv/app/data")
	require.True(t, ok)
	assert.Equal(t, "fs#1", h.rid, "a head merged into fs#1")
	h, ok = containingHead(heads, "/srv/app/logs/old")
	require.True(t, ok)
	assert.Equal(t, "fs#2", h.rid, "the deepest head")
	_, ok = containingHead(heads, "/srv/application")
	assert.False(t, ok, "a prefix of a name is not a head")
	_, ok = containingHead(heads, "/etc")
	assert.False(t, ok)
	assert.Equal(t, []string{"logs"}, nestedHeads(heads, "/srv/app"))
	assert.Empty(t, nestedHeads(heads, "/srv/app/logs"))

	dir := t.TempDir()
	for _, p := range []string{"a", "logs/x", "sub/deep/y", "sub/z"} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, p), nil, 0644))
	}
	require.NoError(t, emptyDir(dir, nil, []string{"logs", "sub/deep"}))
	assert.NoFileExists(t, filepath.Join(dir, "a"))
	assert.NoFileExists(t, filepath.Join(dir, "sub/z"))
	assert.FileExists(t, filepath.Join(dir, "logs/x"), "a head under it")
	assert.FileExists(t, filepath.Join(dir, "sub/deep/y"), "a head deeper under it")
}

// The restore writes as root: a symlink between the head and where a member
// restores, which a process of the object could have made, is refused.
func TestNoSymlinkBetween(t *testing.T) {
	headDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(headDir, "data"), 0755))
	require.NoError(t, os.Symlink("/etc", filepath.Join(headDir, "conf")))
	assert.NoError(t, noSymlinkBetween(headDir, filepath.Join(headDir, "data", "sub")))
	assert.NoError(t, noSymlinkBetween(headDir, filepath.Join(headDir, "new", "dir")), "created by the restore")
	assert.ErrorContains(t, noSymlinkBetween(headDir, filepath.Join(headDir, "conf", "x")), "is a symlink")
}

// The target of a restore in place is created and opened through the head: a
// symlink out of the head is refused, and one a process writing in the head
// swaps in after the target is opened does not move the restore, which
// writes into the directory opened.
func TestOpenUnderStaysInTheHead(t *testing.T) {
	base := t.TempDir()
	headDir := filepath.Join(base, "head")
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.MkdirAll(headDir, 0755))
	require.NoError(t, os.MkdirAll(outside, 0755))
	require.NoError(t, os.Symlink("../outside", filepath.Join(headDir, "escape")))

	_, err := openUnder(headDir, filepath.Join(headDir, "escape", "data"))
	require.Error(t, err, "a symlink out of the head is not followed")
	_, err = os.Stat(filepath.Join(outside, "data"))
	assert.True(t, os.IsNotExist(err), "nothing created outside the head")

	// A symlink staying in the head is refused too: it could point at the
	// head of another resource under this one.
	require.NoError(t, os.MkdirAll(filepath.Join(headDir, "other"), 0755))
	require.NoError(t, os.Symlink("other", filepath.Join(headDir, "inner")))
	_, err = openUnder(headDir, filepath.Join(headDir, "inner", "data"))
	require.Error(t, err, "a symlink in the head is not followed")
	assert.NoDirExists(t, filepath.Join(headDir, "other", "data"))

	f, err := openUnder(headDir, filepath.Join(headDir, "app", "data"))
	require.NoError(t, err)
	defer f.Close()

	// The process writing in the head swaps the target for a symlink out.
	require.NoError(t, os.Rename(filepath.Join(headDir, "app"), filepath.Join(headDir, "moved")))
	require.NoError(t, os.Symlink("../outside", filepath.Join(headDir, "app")))

	dst := fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), f.Fd())
	require.NoError(t, os.WriteFile(filepath.Join(dst, "restored"), []byte("x"), 0600))
	assert.FileExists(t, filepath.Join(headDir, "moved", "data", "restored"), "written into the directory opened")
	assert.NoFileExists(t, filepath.Join(outside, "restored"))
}

// The head itself is reached from the root directory with no symlink
// followed: a process writing in an outer head swapping a component of the
// path of this one for a symlink does not move the restore.
func TestOpenUnderRefusesAHeadPathSwapped(t *testing.T) {
	base := t.TempDir()
	outer := filepath.Join(base, "outer")
	headDir := filepath.Join(outer, "inner", "head")
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.MkdirAll(headDir, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "head"), 0755))

	f, err := openUnder(headDir, filepath.Join(headDir, "data"))
	require.NoError(t, err)
	_ = f.Close()
	assert.DirExists(t, filepath.Join(headDir, "data"))

	// The process writing in the outer head swaps the head's parent for a
	// symlink out, after the checks.
	require.NoError(t, os.Rename(filepath.Join(outer, "inner"), filepath.Join(outer, "moved")))
	require.NoError(t, os.Symlink(outside, filepath.Join(outer, "inner")))
	_, err = openUnder(headDir, filepath.Join(headDir, "data"))
	assert.ErrorContains(t, err, "is a symlink")
	assert.NoDirExists(t, filepath.Join(outside, "head", "data"), "nothing created outside")
}
