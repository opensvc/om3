package rescontainerlxc

import (
	iofs "io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestInitName checks the name of an init is the one of its executable, as
// systemd started as /sbin/init keeps the comm init until it renames itself,
// and the comm when the executable is not readable.
func TestInitName(t *testing.T) {
	root := t.TempDir()
	proc := func(pid, exe, comm string) {
		dir := filepath.Join(root, pid)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		if exe != "" {
			require.NoError(t, os.Symlink(exe, filepath.Join(dir, "exe")))
		} else {
			// Not a link, as the exe of a process not traced is not readable.
			require.NoError(t, os.WriteFile(filepath.Join(dir, "exe"), nil, 0o644))
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, "comm"), []byte(comm+"\n"), 0o644))
	}
	proc("10", "/usr/lib/systemd/systemd", "init")
	proc("11", "/usr/lib/systemd/systemd (deleted)", "systemd")
	proc("12", "/sbin/runit", "init")
	proc("13", "", "systemd")

	for pid, want := range map[int]string{10: "systemd", 11: "systemd", 12: "runit", 13: "systemd"} {
		name, err := initName(root, pid)
		require.NoError(t, err, "pid %d", pid)
		require.Equal(t, want, name, "pid %d", pid)
	}

	_, err := initName(root, 14)
	require.ErrorIs(t, err, iofs.ErrNotExist, "a process gone")
}
