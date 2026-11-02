package volfile

import (
	"errors"
	"fmt"
	"io"
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
	require.NoError(t, Write(head, ".", "a.img", strings.NewReader("one"), -1, Upload{Replace: true}))
	require.NoError(t, Write(head, ".", "a.img", strings.NewReader("two"), -1, Upload{Replace: true}), "in place of the former one")
	b, err := os.ReadFile(filepath.Join(head, "a.img"))
	require.NoError(t, err)
	assert.Equal(t, "two", string(b))

	l, err := List(head, ".")
	require.NoError(t, err)
	require.Len(t, l.Items, 1)
	assert.Equal(t, "a.img", l.Items[0].Name)
	assert.Equal(t, TypeFile, l.Items[0].Type)
	assert.Equal(t, int64(3), l.Items[0].Size)
	assert.False(t, l.Truncated)

	require.NoError(t, Remove(head, ".", "a.img"))
	assert.Error(t, Remove(head, ".", "a.img"))
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
	require.NoError(t, Write(head, ".", "a.img", strings.NewReader("whole"), -1, Upload{Replace: true}))
	assert.Error(t, Write(head, ".", "a.img", &failingReader{}, -1, Upload{Replace: true}))
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
	require.NoError(t, Write(head, ".", "a.img", strings.NewReader("overwritten"), -1, Upload{Replace: true}))
	b, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "secret", string(b), "the link is replaced, not followed")
	info, err := os.Lstat(filepath.Join(head, "a.img"))
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular())

	require.NoError(t, os.Symlink(outside, filepath.Join(head, "link")))
	l, err := List(head, ".")
	require.NoError(t, err)
	require.Len(t, l.Items, 2)
	assert.Equal(t, TypeLink, l.Items[1].Type)
	assert.Equal(t, outside, l.Items[1].Target, "a link is listed as what it says, not followed")
	assert.Equal(t, int64(len(outside)), l.Items[1].Size)
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

	require.NoError(t, Write(head, ".", "a.img", strings.NewReader("x"), -1, Upload{Replace: true}))
	assert.NoFileExists(t, stale)
	assert.FileExists(t, running)
}

// The size of a volume bounds what it is given, its pool being a directory of
// the node no filesystem bounds included.
func TestWriteWithinItsSize(t *testing.T) {
	head := t.TempDir()
	require.NoError(t, Write(head, ".", "a", strings.NewReader("12345"), 5, Upload{}))
	assert.ErrorIs(t, Write(head, ".", "b", strings.NewReader("123456"), 5, Upload{}), ErrFull)
	l, err := os.ReadDir(head)
	require.NoError(t, err)
	assert.Len(t, l, 1, "nothing of what did not fit is left")

	require.NoError(t, Write(head, ".", "a", strings.NewReader("1234567"), 7, Upload{Replace: true}), "a file replaced gives back its place")
	assert.ErrorIs(t, Write(head, ".", "b", strings.NewReader("12"), 8, Upload{}), ErrFull, "what the other files leave")
	require.NoError(t, Write(head, ".", "b", strings.NewReader("1"), 8, Upload{}))
	assert.ErrorIs(t, Write(head, ".", "c", strings.NewReader("1"), 2, Upload{}), ErrFull)
}

// Two transfers count what each other wrote: they run in processes of their
// own, and each would otherwise count on the room there was when it began.
func TestWriteSeesTheOtherTransfers(t *testing.T) {
	head := t.TempDir()
	pr, pw := io.Pipe()
	first := make(chan error, 1)
	go func() { first <- Write(head, ".", "a", pr, 5, Upload{}) }()
	_, err := pw.Write([]byte("123"))
	require.NoError(t, err)
	// pw.Write returns with the bytes read, which are written before the next read.
	require.Eventually(t, func() bool {
		l, err := filepath.Glob(filepath.Join(head, uploadPrefix+"*"))
		if err != nil || len(l) != 1 {
			return false
		}
		info, err := os.Stat(l[0])
		return err == nil && info.Size() == 3
	}, 5*time.Second, time.Millisecond)

	assert.ErrorIs(t, Write(head, ".", "b", strings.NewReader("123"), 5, Upload{}), ErrFull, "the first took the room")
	require.NoError(t, Write(head, ".", "c", strings.NewReader("12"), 5, Upload{}), "there is room for this one")

	_, err = pw.Write([]byte("4"))
	require.NoError(t, err)
	require.NoError(t, pw.Close())
	assert.ErrorIs(t, <-first, ErrFull, "the other ended while the first was going")
}

// An upload does not replace a file it is not told to, an application's
// say, even one created while it was going.
func TestWriteDoesNotReplace(t *testing.T) {
	head := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(head, "app.db"), []byte("data"), 0o600))
	assert.ErrorIs(t, Write(head, ".", "app.db", strings.NewReader("upload"), -1, Upload{}), ErrExists)
	b, err := os.ReadFile(filepath.Join(head, "app.db"))
	require.NoError(t, err)
	assert.Equal(t, "data", string(b))
	l, err := os.ReadDir(head)
	require.NoError(t, err)
	assert.Len(t, l, 1, "the transfer leaves nothing")

	exists, err := Exists(head, ".", "app.db")
	require.NoError(t, err)
	assert.True(t, exists)
	exists, err = Exists(head, ".", "none")
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, Write(head, ".", "app.db", strings.NewReader("upload"), -1, Upload{Replace: true}))
	b, err = os.ReadFile(filepath.Join(head, "app.db"))
	require.NoError(t, err)
	assert.Equal(t, "upload", string(b))

	// Created as the transfer went.
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- Write(head, ".", "late", pr, -1, Upload{}) }()
	_, err = pw.Write([]byte("upload"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(head, "late"), []byte("app"), 0o600))
	require.NoError(t, pw.Close())
	assert.ErrorIs(t, <-done, ErrExists)
	b, err = os.ReadFile(filepath.Join(head, "late"))
	require.NoError(t, err)
	assert.Equal(t, "app", string(b))
}

// A file uploaded says who sent it and when, a file of the application nothing.
func TestListSaysTheUpload(t *testing.T) {
	head := t.TempDir()
	if err := probeXattr(head); err != nil {
		t.Skipf("no user extended attributes here: %s", err)
	}
	before := time.Now().Add(-time.Second)
	require.NoError(t, Write(head, ".", "up.iso", strings.NewReader("x"), -1, Upload{By: "alice"}))
	require.NoError(t, os.WriteFile(filepath.Join(head, "app.db"), []byte("y"), 0o600))
	l, err := List(head, ".")
	require.NoError(t, err)
	byName := map[string]Info{}
	for _, f := range l.Items {
		byName[f.Name] = f
	}
	assert.Equal(t, "alice", byName["up.iso"].UploadedBy)
	assert.True(t, byName["up.iso"].UploadedAt.After(before))
	assert.Empty(t, byName["app.db"].UploadedBy)
	assert.True(t, byName["app.db"].UploadedAt.IsZero())

	// Rewritten in place, it keeps the mark: modified_at tells it changed.
	f, err := os.OpenFile(filepath.Join(head, "up.iso"), os.O_WRONLY|os.O_TRUNC, 0)
	require.NoError(t, err)
	_, err = f.WriteString("changed")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	l, err = List(head, ".")
	require.NoError(t, err)
	for _, f := range l.Items {
		if f.Name == "up.iso" {
			assert.Equal(t, "alice", f.UploadedBy)
		}
	}
}

func TestCleanDir(t *testing.T) {
	for in, want := range map[string]string{"": ".", ".": ".", "a": "a", "a/b": "a/b", "a/b/": "a/b", "a//b": "a/b", "./a": "a"} {
		got, err := CleanDir(in)
		require.NoError(t, err, "%q", in)
		assert.Equal(t, want, got, "%q", in)
	}
	for _, in := range []string{"/", "/etc", "..", "../x", "a/../b", "a/..", ".hidden", "a/.git", `a\b`, "a\x00b"} {
		_, err := CleanDir(in)
		assert.ErrorIs(t, err, ErrInvalidDir, "%q", in)
	}
}

// The directories of a volume are listed, made, written to and removed, and
// none leads out of it.
func TestDirectories(t *testing.T) {
	head := t.TempDir()
	require.NoError(t, Mkdir(head, ".", "isos"))
	assert.ErrorIs(t, Mkdir(head, ".", "isos"), ErrExists)
	require.NoError(t, Mkdir(head, "isos", "debian"))
	assert.ErrorIs(t, Mkdir(head, "none", "x"), ErrNoDir, "no parent made")

	require.NoError(t, Write(head, "isos", "a.iso", strings.NewReader("iso"), -1, Upload{By: "alice"}))
	assert.ErrorIs(t, Write(head, "isos", "a.iso", strings.NewReader("iso"), -1, Upload{}), ErrExists)
	b, err := os.ReadFile(filepath.Join(head, "isos", "a.iso"))
	require.NoError(t, err)
	assert.Equal(t, "iso", string(b))
	assert.ErrorIs(t, Write(head, "none", "a.iso", strings.NewReader("iso"), -1, Upload{}), ErrNoDir)
	assert.ErrorIs(t, Write(head, "isos/a.iso", "b", strings.NewReader("iso"), -1, Upload{}), ErrNotDir)

	exists, err := Exists(head, "isos", "a.iso")
	require.NoError(t, err)
	assert.True(t, exists)

	require.NoError(t, os.Symlink("a.iso", filepath.Join(head, "isos", "latest.iso")))
	require.NoError(t, os.WriteFile(filepath.Join(head, "isos", ".hidden"), nil, 0o600))
	l, err := List(head, "isos")
	require.NoError(t, err)
	got := map[string]Info{}
	for _, f := range l.Items {
		got[f.Name] = f
	}
	require.Len(t, got, 3, "the hidden left out")
	assert.Equal(t, TypeFile, got["a.iso"].Type)
	assert.Equal(t, TypeDir, got["debian"].Type)
	assert.Equal(t, int64(0), got["debian"].Size)
	assert.Equal(t, TypeLink, got["latest.iso"].Type)
	assert.Equal(t, "a.iso", got["latest.iso"].Target)
	if probeXattr(head) == nil {
		assert.Equal(t, "alice", got["a.iso"].UploadedBy, "marked in a subdirectory too")
	}
	assert.Equal(t, []string{"a.iso", "debian", "latest.iso"}, []string{l.Items[0].Name, l.Items[1].Name, l.Items[2].Name}, "by name")

	_, err = List(head, "none")
	assert.ErrorIs(t, err, ErrNoDir)
	_, err = List(head, "isos/a.iso")
	assert.ErrorIs(t, err, ErrNotDir)

	// A link leading out of the volume is no way out.
	require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(head, "out")))
	_, err = List(head, "out")
	assert.ErrorIs(t, err, ErrInvalidDir)
	assert.Error(t, Write(head, "out", "x", strings.NewReader("x"), -1, Upload{}))
	assert.Error(t, Mkdir(head, "out", "x"))

	assert.ErrorIs(t, Remove(head, ".", "isos"), ErrNotEmpty, "nothing recursive")
	require.NoError(t, Remove(head, "isos", "latest.iso"), "the link, not what it leads to")
	assert.FileExists(t, filepath.Join(head, "isos", "a.iso"))
	require.NoError(t, Remove(head, "isos", "debian"))
	require.NoError(t, Remove(head, "isos", "a.iso"))
	assert.ErrorIs(t, Remove(head, ".", "isos"), ErrNotEmpty, "a hidden file is something")
	require.NoError(t, os.Remove(filepath.Join(head, "isos", ".hidden")))
	require.NoError(t, Remove(head, ".", "isos"))
}

// What the subdirectories hold counts in the size of the volume.
func TestWriteCountsTheSubdirectories(t *testing.T) {
	head := t.TempDir()
	require.NoError(t, Mkdir(head, ".", "a"))
	require.NoError(t, os.WriteFile(filepath.Join(head, "a", "big"), []byte("12345"), 0o600))
	assert.ErrorIs(t, Write(head, ".", "b", strings.NewReader("12"), 6, Upload{}), ErrFull)
	require.NoError(t, Write(head, "a", "c", strings.NewReader("1"), 6, Upload{}))
	require.NoError(t, Write(head, "a", "big", strings.NewReader("12"), 6, Upload{Replace: true}), "a file replaced gives back its place")
}

func TestListIsCapped(t *testing.T) {
	head := t.TempDir()
	for i := 0; i <= MaxList; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(head, fmt.Sprintf("f%04d", i)), nil, 0o600))
	}
	l, err := List(head, ".")
	require.NoError(t, err)
	assert.Len(t, l.Items, MaxList)
	assert.True(t, l.Truncated)
	assert.Equal(t, "f0000", l.Items[0].Name)
}
