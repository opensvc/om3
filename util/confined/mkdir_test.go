package confined

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The missing directories are created from the outermost, each handed to made,
// so that each one can get its full mode: an existing one is left as it is.
func TestMkdirAllEachCreatesAndHandsOverEachMissingDirectory(t *testing.T) {
	head := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(head, "a"), 0o700))
	tree, err := Open(head)
	require.NoError(t, err)
	defer func() { _ = tree.Close() }()

	want := 0o750 | os.ModeSetgid
	var made []string
	err = MkdirAllEach(tree, filepath.Join(head, "a", "b", "c"), want, func(p string) error {
		made = append(made, p)
		return tree.Chmod(p, want)
	})
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(head, "a", "b"), filepath.Join(head, "a", "b", "c")}, made)
	for _, p := range made {
		info, err := os.Stat(p)
		require.NoError(t, err)
		assert.Equal(t, want, info.Mode()&(os.ModePerm|os.ModeSetgid), p)
	}
	info, err := os.Stat(filepath.Join(head, "a"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm(), "an existing directory is left as it is")

	made = nil
	require.NoError(t, MkdirAllEach(tree, filepath.Join(head, "a", "b", "c"), want, func(p string) error {
		made = append(made, p)
		return nil
	}))
	assert.Empty(t, made, "nothing is created twice")
}
