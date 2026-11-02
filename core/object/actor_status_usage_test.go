package object

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalkUsage(t *testing.T) {
	head := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(head, "a", "b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(head, "a", "b", "f"), make([]byte, 64*1024), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(head, "g"), make([]byte, 64*1024), 0o600))
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "big"), make([]byte, 4*1024*1024), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(head, "out")))

	used, err := walkUsage(head, time.Now().Add(time.Minute))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, used, int64(128*1024), "the files of the subdirectories count, as their blocks")
	assert.Less(t, used, int64(4*1024*1024), "a link is not followed to the 4 MiB it leads to")

	_, err = walkUsage(head, time.Now().Add(-time.Second))
	assert.ErrorIs(t, err, errUsageWalkBudget)
}

func TestIsMountPoint(t *testing.T) {
	assert.True(t, isMountPoint("/"))
	dir := t.TempDir()
	assert.False(t, isMountPoint(dir), "a directory of the filesystem of its parent")
	assert.False(t, isMountPoint(filepath.Join(dir, "none")))
}
