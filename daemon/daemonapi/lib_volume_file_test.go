package daemonapi

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsVolumeFileName(t *testing.T) {
	assert.True(t, isVolumeFileName("noble.qcow2"))
	for _, s := range []string{"", ".", "..", "../x", "a/b", "/etc/passwd", ".hidden", `a\b`, "a\x00b", strings.Repeat("a", 256)} {
		assert.False(t, isVolumeFileName(s), "%q", s)
	}
}

func TestWriteVolumeFile(t *testing.T) {
	head := t.TempDir()
	require.NoError(t, writeVolumeFile(head, "a.img", strings.NewReader("one"), -1))
	require.NoError(t, writeVolumeFile(head, "a.img", strings.NewReader("two"), -1), "in place of the former one")
	b, err := os.ReadFile(filepath.Join(head, "a.img"))
	require.NoError(t, err)
	assert.Equal(t, "two", string(b))

	l, err := listVolumeFiles(head)
	require.NoError(t, err)
	require.Len(t, l, 1)
	assert.Equal(t, "a.img", l[0].Name)
	assert.Equal(t, int64(3), l[0].Size)

	require.NoError(t, removeVolumeFile(head, "a.img"))
	assert.Error(t, removeVolumeFile(head, "a.img"))
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
func TestWriteVolumeFileFailing(t *testing.T) {
	head := t.TempDir()
	require.NoError(t, writeVolumeFile(head, "a.img", strings.NewReader("whole"), -1))
	assert.Error(t, writeVolumeFile(head, "a.img", &failingReader{}, -1))
	b, err := os.ReadFile(filepath.Join(head, "a.img"))
	require.NoError(t, err)
	assert.Equal(t, "whole", string(b))
	l, err := os.ReadDir(head)
	require.NoError(t, err)
	assert.Len(t, l, 1)
}

// What a volume holds is its user's: a link leading out of it is no way to the
// files of the node, for a write as root.
func TestWriteVolumeFileStaysInTheVolume(t *testing.T) {
	head := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))

	// The temporary name is known to whoever holds the volume.
	require.NoError(t, os.Symlink(outside, filepath.Join(head, uploadPrefix+"a.img")))
	assert.Error(t, writeVolumeFile(head, "a.img", strings.NewReader("overwritten"), -1))
	b, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "secret", string(b))

	require.NoError(t, os.Remove(filepath.Join(head, uploadPrefix+"a.img")))
	require.NoError(t, os.Symlink(outside, filepath.Join(head, "link")))
	l, err := listVolumeFiles(head)
	require.NoError(t, err)
	assert.Empty(t, l, "a link is not listed")
}

// The size of a volume bounds what it is given, its pool being a directory of
// the node no filesystem bounds included.
func TestWriteVolumeFileWithinItsSize(t *testing.T) {
	head := t.TempDir()
	require.NoError(t, writeVolumeFile(head, "a", strings.NewReader("12345"), 5))
	assert.ErrorIs(t, writeVolumeFile(head, "b", strings.NewReader("123456"), 5), errVolumeFull)
	l, err := os.ReadDir(head)
	require.NoError(t, err)
	assert.Len(t, l, 1, "nothing of what did not fit is left")

	n, err := room(head, "b", 8)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n, "what the other files leave")
	n, err = room(head, "a", 8)
	require.NoError(t, err)
	assert.Equal(t, int64(8), n, "a file replaced gives back its place")
	n, err = room(head, "b", 2)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}
