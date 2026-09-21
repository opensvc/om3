package imon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A resource file lives where the resource that holds it lives: the
// configuration of a container in the filesystem that carries the container.
// A node that does not run the object does not hold that filesystem, so there
// is nowhere to fetch into, and asking the peer for the file every time it
// refreshes its status asks for something that cannot land.
func TestAFileIsFetchedOnlyWhereItCanLand(t *testing.T) {
	held := t.TempDir()

	dir, ok := fetchDir(filepath.Join(held, "config"))
	assert.True(t, ok, "the directory is there, so the file has somewhere to go")
	assert.Equal(t, held, dir)

	dir, ok = fetchDir(filepath.Join(held, "base", "c12lxc2", "config"))
	assert.False(t, ok, "the filesystem carrying it is mounted where the object runs")
	assert.Equal(t, filepath.Join(held, "base", "c12lxc2"), dir)

	// A file whose directory is a file is not one to write into either.
	require.NoError(t, os.WriteFile(filepath.Join(held, "notadir"), []byte("x"), 0600))
	_, ok = fetchDir(filepath.Join(held, "notadir", "config"))
	assert.False(t, ok)
}

// The reason is said once: the status that triggers the fetch arrives every
// time the peer refreshes it, and the object runs there for as long as it
// runs there.
func TestTheReasonAFileIsNotFetchedIsSaidOnce(t *testing.T) {
	m := newFilesManager()
	assert.False(t, m.skipped["/srv/x/config"])
	m.skipped["/srv/x/config"] = true
	assert.True(t, m.skipped["/srv/x/config"], "said once, and not again")
	delete(m.skipped, "/srv/x/config")
	assert.False(t, m.skipped["/srv/x/config"], "and said again once it has been there and gone")
}
