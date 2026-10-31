package volfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsName(t *testing.T) {
	assert.True(t, IsName("noble.qcow2"))
	for _, s := range []string{"", ".", "..", "../x", "a/b", "/etc/passwd", ".hidden", `a\b`, "a\x00b", strings.Repeat("a", 256)} {
		assert.False(t, IsName(s), "%q", s)
	}
}

func TestWrite(t *testing.T) {
	head := t.TempDir()
	require.NoError(t, Write(head, "a.img", strings.NewReader("one"), -1))
	require.NoError(t, Write(head, "a.img", strings.NewReader("two"), -1), "in place of the former one")
	b, err := os.ReadFile(filepath.Join(head, "a.img"))
	require.NoError(t, err)
	assert.Equal(t, "two", string(b))

	l, err := List(head)
	require.NoError(t, err)
	require.Len(t, l, 1)
	assert.Equal(t, "a.img", l[0].Name)
	assert.Equal(t, int64(3), l[0].Size)

	require.NoError(t, Remove(head, "a.img"))
	assert.Error(t, Remove(head, "a.img"))
}

type failingReader struct{ read bool }

func (t *failingReader) Read(p []byte) (int, error) {
	if t.read {
		return 0, errors.New("connection lost")
	}
	t.read = true
	return copy(p, "part"), nil
}

// A transfer that fails leaves what was there, and nothing of itself.
func TestWriteFailing(t *testing.T) {
	head := t.TempDir()
	require.NoError(t, Write(head, "a.img", strings.NewReader("whole"), -1))
	assert.Error(t, Write(head, "a.img", &failingReader{}, -1))
	b, err := os.ReadFile(filepath.Join(head, "a.img"))
	require.NoError(t, err)
	assert.Equal(t, "whole", string(b))
	l, err := os.ReadDir(head)
	require.NoError(t, err)
	assert.Len(t, l, 1)
}

// What a volume holds is its user's: a link leading out of it is no way to the
// files of the node, for a write as root.
func TestWriteStaysInTheVolume(t *testing.T) {
	head := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))

	require.NoError(t, os.Symlink(outside, filepath.Join(head, "a.img")))
	require.NoError(t, Write(head, "a.img", strings.NewReader("overwritten"), -1))
	b, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "secret", string(b), "the link is replaced, not followed")
	info, err := os.Lstat(filepath.Join(head, "a.img"))
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular())

	require.NoError(t, os.Symlink(outside, filepath.Join(head, "link")))
	l, err := List(head)
	require.NoError(t, err)
	assert.Len(t, l, 1, "a link is not listed")
}

// What a transfer killed left is removed, for the volume to count what it
// holds. One running is left alone.
func TestWriteSweepsWhatKilledTransfersLeft(t *testing.T) {
	head := t.TempDir()
	stale := filepath.Join(head, uploadPrefix+"old")
	running := filepath.Join(head, uploadPrefix+"running")
	require.NoError(t, os.WriteFile(stale, []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(running, []byte("x"), 0o644))
	old := time.Now().Add(-2 * staleUpload)
	require.NoError(t, os.Chtimes(stale, old, old))

	require.NoError(t, Write(head, "a.img", strings.NewReader("x"), -1))
	assert.NoFileExists(t, stale)
	assert.FileExists(t, running)
}

// The size of a volume bounds what it is given, its pool being a directory of
// the node no filesystem bounds included.
func TestWriteWithinItsSize(t *testing.T) {
	head := t.TempDir()
	require.NoError(t, Write(head, "a", strings.NewReader("12345"), 5))
	assert.ErrorIs(t, Write(head, "b", strings.NewReader("123456"), 5), ErrFull)
	l, err := os.ReadDir(head)
	require.NoError(t, err)
	assert.Len(t, l, 1, "nothing of what did not fit is left")

	n, err := Room(head, "b", 8)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n, "what the other files leave")
	n, err = Room(head, "a", 8)
	require.NoError(t, err)
	assert.Equal(t, int64(8), n, "a file replaced gives back its place")
	n, err = Room(head, "b", 2)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}
