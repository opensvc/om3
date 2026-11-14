//go:build linux

package df

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installDF puts first in the PATH a df recording its arguments to the
// returned file, and printing one entry, as GNU df does for a path.
func installDF(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	argsFile := filepath.Join(d, "args")
	script := `#!/bin/sh
echo "$@" >>` + argsFile + `
echo "Filesystem 1024-blocks Used Available Capacity Mounted on"
echo "srv:/export 1000 400 600 40% /mnt/share"
`
	require.NoError(t, os.WriteFile(filepath.Join(d, "df"), []byte(script), 0o755))
	t.Setenv("PATH", d+":"+os.Getenv("PATH"))
	return argsFile
}

func readArgs(t *testing.T, argsFile string) []string {
	t.Helper()
	b, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// The usage of a path is read whatever its filesystem, local or remote, as a
// share pool on nfs: GNU df -l fails for a path on a remote filesystem with
// "no file systems processed". The listings keep to the local filesystems.
func TestPathUsageIsNotLimitedToLocalFilesystems(t *testing.T) {
	argsFile := installDF(t)
	dir := t.TempDir()

	entries, err := ContainingMountUsage(t.Context(), filepath.Join(dir, "not", "yet"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "/mnt/share", entries[0].MountPoint)
	assert.Equal(t, int64(600*1024), entries[0].Free)

	_, err = MountUsage(t.Context(), dir)
	require.NoError(t, err)
	_, err = TypeMountUsage(t.Context(), "nfs", dir)
	require.NoError(t, err)
	_, err = Usage(t.Context())
	require.NoError(t, err)
	_, err = Inode(t.Context())
	require.NoError(t, err)

	assert.Equal(t, []string{
		"-P " + dir,
		"-P " + dir,
		"-P -t nfs " + dir,
		"-lP",
		"-lPi",
	}, readArgs(t, argsFile))
}
