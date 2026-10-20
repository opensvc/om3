package ressyncstore

import (
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

	require.NoError(t, emptyDir(dir, []string{"/tmp", "*.log", "data/cache"}))
	assert.NoFileExists(t, filepath.Join(dir, "a"))
	assert.NoFileExists(t, filepath.Join(dir, "sub", "b"))
	assert.FileExists(t, filepath.Join(dir, "sub", "c.log"))
	assert.FileExists(t, filepath.Join(dir, "tmp", "x"))
	assert.FileExists(t, filepath.Join(dir, "data", "cache", "y"))
	assert.NoFileExists(t, filepath.Join(dir, "data", "z"))
	assert.NoFileExists(t, filepath.Join(dir, "link"))
	assert.FileExists(t, filepath.Join(outside, "keep"), "a symlink is removed, not followed")

	require.NoError(t, emptyDir(dir, nil))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
