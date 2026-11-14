//go:build unix

package confined

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The directories made are given the mode asked, whatever the umask, and the
// directory the walk starts from is not checked: it is the system's.
func TestOpenDirTrustedMakes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o1777))
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)

	r, err := OpenDirTrusted(dir, "opensvc/ns1/svc", true, 0o755, nil)
	require.NoError(t, err)
	_ = r.Close()
	for _, p := range []string{"opensvc", "opensvc/ns1", "opensvc/ns1/svc"} {
		fi, err := os.Stat(filepath.Join(dir, p))
		require.NoError(t, err)
		assert.Equal(t, fs.FileMode(0o755), fi.Mode().Perm(), p)
	}

	_, err = OpenDirTrusted(dir, "opensvc/ns2/svc", false, 0, nil)
	require.ErrorIs(t, err, os.ErrNotExist, "nothing made without create")
}

// A directory of the user others can write in is given the mode asked, and
// said so.
func TestOpenDirTrustedTightens(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "opensvc")
	require.NoError(t, os.Mkdir(p, 0o755))
	require.NoError(t, os.Chmod(p, 0o777))
	var told []string
	require.NoError(t, MkdirTrusted(dir, "opensvc/svc", 0o755, func(path string, mode fs.FileMode) {
		told = append(told, path+" "+mode.String())
	}))
	assert.Equal(t, []string{p + " -rwxrwxrwx"}, told)
	fi, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o755), fi.Mode().Perm())
}

// A link on the way is refused, and nothing is made where it leads.
func TestOpenDirTrustedRefusesALink(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "opensvc")))
	_, err := OpenDirTrusted(dir, "opensvc/svc", true, 0o755, nil)
	require.ErrorContains(t, err, "is a link")
	assert.NoDirExists(t, filepath.Join(target, "svc"))
}

// A directory of another user is refused, never taken over: its mode and
// its owner are left as they are.
func TestOpenDirTrustedRefusesAnotherUser(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root to give a directory to another user")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "opensvc")
	require.NoError(t, os.Mkdir(p, 0o777))
	require.NoError(t, os.Chown(p, 65534, 65534))
	_, err := OpenDirTrusted(dir, "opensvc/svc", true, 0o755, nil)
	require.ErrorIs(t, err, ErrUntrusted)
	assert.NoDirExists(t, filepath.Join(p, "svc"))
	fi, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, uint32(65534), fi.Sys().(*syscall.Stat_t).Uid)
}

// A trusted file is a regular file of the user: a link, a directory, a file
// of another user are not, and a missing file is not, with no error.
func TestTrustedFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "own"), nil, 0o644))
	require.NoError(t, os.Symlink("own", filepath.Join(dir, "link")))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "dir"), 0o755))
	r, err := os.OpenRoot(dir)
	require.NoError(t, err)
	defer r.Close()

	ok, err := TrustedFile(r, "own")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = TrustedFile(r, "missing")
	require.NoError(t, err)
	assert.False(t, ok)

	for _, name := range []string{"link", "dir"} {
		ok, err = TrustedFile(r, name)
		require.ErrorIs(t, err, ErrUntrusted, name)
		assert.False(t, ok, name)
	}

	if os.Geteuid() == 0 {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "other"), nil, 0o644))
		require.NoError(t, os.Chown(filepath.Join(dir, "other"), 65534, 65534))
		ok, err = TrustedFile(r, "other")
		require.ErrorIs(t, err, ErrUntrusted)
		assert.False(t, ok)
	}
}
