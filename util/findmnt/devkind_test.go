package findmnt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDevKind(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "tank", "fs"), 0755))
	file := filepath.Join(dir, "img")
	require.NoError(t, os.WriteFile(file, nil, 0644))
	t.Chdir(dir)

	isDir, isRegular, isNfs := devKind("tank/fs")
	require.False(t, isDir, "a dataset name is no path, whatever the working directory holds")
	require.False(t, isRegular)
	require.False(t, isNfs)

	isDir, _, _ = devKind(filepath.Join(dir, "tank", "fs"))
	require.True(t, isDir, "an absolute directory is a bind source")

	_, isRegular, _ = devKind(file)
	require.True(t, isRegular)

	_, _, isNfs = devKind("host:/export")
	require.True(t, isNfs)

	isDir, isRegular, isNfs = devKind("/does/not/exist")
	require.False(t, isDir || isRegular || isNfs)
}
