package storeplakar

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUsageMeasuresTheDirectoryOfTheStore pins the space a store tells: the
// size of the files of its directory and the space left in its filesystem,
// and nothing of a remote location, plakar not asked.
func TestUsageMeasuresTheDirectoryOfTheStore(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()

	// Not written yet: it takes nothing, where it will be has space.
	u, err := e.store.Usage(ctx)
	require.NoError(t, err)
	assert.Nil(t, u.StoredBytes)
	require.NotNil(t, u.FreeBytes, "a filesystem that is full has none, and says so")

	e.createStore(t)
	require.NoError(t, os.MkdirAll(filepath.Join(e.storePath(), "packfiles", "ab"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(e.storePath(), "packfiles", "ab", "pack"), make([]byte, 4096), 0600))
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(e.storePath(), "link")))
	u, err = e.store.Usage(ctx)
	require.NoError(t, err)
	require.NotNil(t, u.StoredBytes)
	assert.Equal(t, int64(4097), *u.StoredBytes, "the pack and CONFIG, the link not followed")
	require.NotNil(t, u.FreeBytes)
	assert.Empty(t, calls(t), "plakar is not asked: no key to derive")

	u, err = urlStore(t, "s3://h/bucket").Usage(ctx)
	require.NoError(t, err)
	assert.Nil(t, u.StoredBytes)
	assert.Nil(t, u.FreeBytes)
	_, err = urlStore(t, "srv/plakar").Usage(ctx)
	require.Error(t, err)
}
