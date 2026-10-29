package rescontainerlxc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPurgeLxcVarInDataDir checks the unprovision removes the directory of
// the container in its container_data_dir, and nothing of the other
// containers there.
func TestPurgeLxcVarInDataDir(t *testing.T) {
	dataDir := t.TempDir()
	for _, name := range []string{"c1", "c2"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dataDir, name, "rootfs"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dataDir, name, "config"), nil, 0o644))
	}
	r := New().(*T)
	r.Name = "c1"
	r.DataDir = dataDir
	ctx := context.Background()

	require.Equal(t, dataDir, r.lxcPath(ctx))
	require.NoError(t, r.unprovision(ctx))
	require.NoDirExists(t, filepath.Join(dataDir, "c1"))
	require.FileExists(t, filepath.Join(dataDir, "c2", "config"))
}
