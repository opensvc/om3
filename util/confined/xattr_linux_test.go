package confined

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// The extended attributes of a file of the tree are the file's, never the
// ones of what a link leads to.
func TestTreeXattr(t *testing.T) {
	dir := t.TempDir()
	probe := filepath.Join(dir, ".probe")
	require.NoError(t, os.WriteFile(probe, nil, 0o600))
	if err := unix.Setxattr(probe, "user.probe", []byte("1"), 0); err != nil {
		t.Skipf("no user extended attributes here: %s", err)
	}
	tree, err := Open(dir)
	require.NoError(t, err)
	f := filepath.Join(dir, "f")
	require.NoError(t, os.WriteFile(f, nil, 0o600))
	require.NoError(t, tree.SetXattr(f, "user.t", []byte("v")))
	b, err := tree.GetXattr(f, "user.t")
	require.NoError(t, err)
	assert.Equal(t, "v", string(b))
	require.NoError(t, tree.RemoveXattr(f, "user.t"))
	_, err = tree.GetXattr(f, "user.t")
	assert.Error(t, err)

	require.NoError(t, os.Symlink("f", filepath.Join(dir, "l")))
	assert.Error(t, tree.SetXattr(filepath.Join(dir, "l"), "user.t", []byte("v")), "a link is not followed")
	_, err = tree.GetXattr(f, "user.t")
	assert.Error(t, err, "nor what it leads to written")
}
