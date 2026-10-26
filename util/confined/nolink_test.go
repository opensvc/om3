package confined

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A link on the way is refused, one leading back into the directory too, and
// nothing is made out of it.
func TestOpenDirNoLink(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "rootfs")
	host := filepath.Join(base, "host")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "srv"), 0755))
	require.NoError(t, os.MkdirAll(host, 0755))

	r, err := OpenDirNoLink(dir, "/srv/data", true, 0755)
	require.NoError(t, err)
	_ = r.Close()
	assert.DirExists(t, filepath.Join(dir, "srv", "data"))

	require.NoError(t, os.Symlink(host, filepath.Join(dir, "etc")))
	_, err = OpenDirNoLink(dir, "/etc/data", true, 0755)
	assert.ErrorContains(t, err, "is a link")
	assert.NoDirExists(t, filepath.Join(host, "data"), "nothing made out of the directory")

	require.NoError(t, os.Symlink("srv", filepath.Join(dir, "inner")))
	_, err = OpenDirNoLink(dir, "/inner/data", false, 0)
	assert.ErrorContains(t, err, "is a link", "a link staying in the directory too")

	_, err = OpenDirNoLink(dir, "/missing/data", false, 0)
	assert.Error(t, err, "a missing component without create")
}

func TestCheckNoLink(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "srv"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "srv", "file"), nil, 0644))
	require.NoError(t, os.Symlink("/", filepath.Join(dir, "etc")))
	assert.NoError(t, CheckNoLink(dir, "/srv/data"), "up to the first missing component")
	assert.NoError(t, CheckNoLink(dir, "/srv/file"), "a file as last component")
	assert.ErrorContains(t, CheckNoLink(dir, "/etc/data"), "is a link")
	assert.ErrorContains(t, CheckNoLink(dir, "/etc"), "is a link", "the last component too")
	assert.NoError(t, CheckNoLink(dir, "/"), "the directory itself")
}
