package confined

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tree makes a tree, and a file out of it a planted link could lead to.
func tree(t *testing.T) (*Tree, string) {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, "head")
	require.NoError(t, os.Mkdir(dir, 0755))
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.WriteFile(outside, []byte("orig"), 0600))
	tr, err := Open(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tr.Close() })
	return tr, outside
}

func TestAWriteThroughAPlantedLinkIsRefused(t *testing.T) {
	tr, outside := tree(t)
	link := filepath.Join(tr.Dir(), "index.html")
	require.NoError(t, os.Symlink(outside, link))

	assert.Error(t, tr.WriteFile(link, []byte("pwned"), 0644))
	assert.Error(t, tr.Chmod(link, 0644))
	b, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "orig", string(b))
	info, err := os.Stat(outside)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestAWriteUnderAPlantedDirectoryLinkIsRefused(t *testing.T) {
	tr, outside := tree(t)
	require.NoError(t, os.Symlink(filepath.Dir(outside), filepath.Join(tr.Dir(), "html")))

	assert.Error(t, tr.WriteFile(filepath.Join(tr.Dir(), "html", "outside"), []byte("pwned"), 0644))
	assert.Error(t, tr.MkdirAll(filepath.Join(tr.Dir(), "html", "sub"), 0755))
	assert.NoDirExists(t, filepath.Join(filepath.Dir(outside), "sub"))
}

func TestAPathNamedOutOfTheTreeIsRefused(t *testing.T) {
	tr, outside := tree(t)
	err := tr.WriteFile(filepath.Join(tr.Dir(), "..", "outside"), []byte("pwned"), 0644)
	assert.True(t, errors.Is(err, ErrEscapes), "%v", err)
	err = tr.Chmod(outside, 0644)
	assert.True(t, errors.Is(err, ErrEscapes), "%v", err)
}

// Lchown changes a link, never what it leads to.
func TestLchownChangesTheLinkItself(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("chown needs root")
	}
	tr, outside := tree(t)
	link := filepath.Join(tr.Dir(), "index.html")
	require.NoError(t, os.Symlink(outside, link))
	require.NoError(t, tr.Lchown(link, 4242, 4242))
	info, err := os.Stat(outside)
	require.NoError(t, err)
	assert.NotEqual(t, uint32(4242), uidOf(info))
}

func TestPathsInTheTreeWork(t *testing.T) {
	tr, _ := tree(t)
	p := filepath.Join(tr.Dir(), "html", "index.html")
	require.NoError(t, tr.MkdirAll(filepath.Dir(p), 0755))
	require.NoError(t, tr.WriteFile(p, []byte("ok"), 0640))
	b, err := tr.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(b))
	assert.Error(t, tr.RemoveAll(tr.Dir()), "the tree itself is not removed")
}

func uidOf(info os.FileInfo) uint32 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return st.Uid
	}
	return 0
}
