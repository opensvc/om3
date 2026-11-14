package listener

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A certificate file is written through a new file: a file someone made in
// its place and holds open never receives the content, a link in its place
// is replaced, not followed, and the file is the daemon user's alone.
func TestWriteCertFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "private_key")

	require.NoError(t, os.WriteFile(p, []byte("planted"), 0o666))
	held, err := os.Open(p)
	require.NoError(t, err)
	defer held.Close()
	require.NoError(t, writeCertFile(p, []byte("secret")))
	b, err := io.ReadAll(held)
	require.NoError(t, err)
	assert.Equal(t, "planted", string(b), "the descriptor held reads the former file")
	b, err = os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "secret", string(b))
	fi, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())

	victim := filepath.Join(t.TempDir(), "victim")
	require.NoError(t, os.WriteFile(victim, []byte("data"), 0o644))
	link := filepath.Join(dir, "ca_certificates")
	require.NoError(t, os.Symlink(victim, link))
	require.NoError(t, writeCertFile(link, []byte("cas")))
	b, err = os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "data", string(b), "the link is not followed")
	fi, err = os.Lstat(link)
	require.NoError(t, err)
	assert.True(t, fi.Mode().IsRegular())

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "no temporary file left")
}

// The directory of the certificate files, as a tmpfs a former daemon left
// mounted mode 1777, is made the daemon user's alone.
func TestSecureCertDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o1777))
	require.NoError(t, secureCertDir(dir))
	fi, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), fi.Mode().Perm())
	assert.Zero(t, fi.Mode()&os.ModeSticky)
}
