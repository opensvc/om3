package resfshost

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The write probe runs as root in a filesystem its users fill: a symlink they
// plant is not followed to a file of the node, a fifo does not hang it, and a
// file found there is not truncated.
func TestTheWriteProbeWritesOnlyInARegularFileOfTheFilesystem(t *testing.T) {
	r := newMountPointResource(t, t.TempDir())
	mnt := t.TempDir()

	victim := filepath.Join(t.TempDir(), "victim")
	require.NoError(t, os.WriteFile(victim, []byte("secret"), 0600))
	link := filepath.Join(mnt, "link")
	require.NoError(t, os.Symlink(victim, link))
	assert.Error(t, r.checkWriteFile(link))
	b, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "secret", string(b), "the file a symlink points at is untouched")

	fifo := filepath.Join(mnt, "fifo")
	require.NoError(t, syscall.Mkfifo(fifo, 0600))
	assert.Error(t, r.checkWriteFile(fifo), "a fifo with no reader")

	probe := filepath.Join(mnt, ".opensvc")
	require.NoError(t, os.WriteFile(probe, []byte("held"), 0600))
	require.NoError(t, r.checkWriteFile(probe))
	b, err = os.ReadFile(probe)
	require.NoError(t, err)
	assert.Equal(t, " eld", string(b), "a byte written, nothing truncated")

	require.NoError(t, r.checkWriteFile(filepath.Join(mnt, "new")))
}
