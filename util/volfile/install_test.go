package volfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// A file fetched from a url is written aside and renamed in place, given its
// mode and the mark of the url, which has no key: an upload does not replace
// it, naming the url to change, and a delete is allowed, the next install
// fetching it again.
func TestInstall(t *testing.T) {
	head := t.TempDir()
	if err := probeXattr(head); err != nil {
		t.Skipf("no user extended attributes here: %s", err)
	}
	require.NoError(t, os.MkdirAll(filepath.Join(head, "web/html"), 0o755))
	url := "https://example.com/releases/latest/download/index.html.gz"
	m := InstallMark{From: url, At: time.Now()}
	require.NoError(t, Install(head, "web/html", "index.html.gz", strings.NewReader("one"), m, 0o640, -1, -1))
	require.NoError(t, Install(head, "web/html", "index.html.gz", strings.NewReader("two"), m, 0o640, -1, -1), "in place of the former one")

	p := filepath.Join(head, "web/html/index.html.gz")
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "two", string(b))
	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())

	buf := make([]byte, 4096)
	n, err := unix.Getxattr(p, InstallAttr, buf)
	require.NoError(t, err)
	assert.NotContains(t, string(buf[:n]), `"key"`, "a url has no key")
	got, ok, err := Installed(head, "web/html", "index.html.gz")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, url, got.From)
	assert.Empty(t, got.Key)

	err = Write(head, "web/html", "index.html.gz", strings.NewReader("edited"), -1, Upload{Replace: true})
	assert.ErrorIs(t, err, ErrInstalled)
	assert.ErrorContains(t, err, "index.html.gz is installed from "+url+": change the source url")

	l, err := os.ReadDir(head)
	require.NoError(t, err)
	assert.Len(t, l, 1, "nothing left aside at the top of the volume")

	require.NoError(t, Remove(head, "web/html", "index.html.gz"), "a delete is allowed: the next install fetches it again")
}

// A fetch that fails leaves the former file as it is, and nothing aside.
func TestInstallFailing(t *testing.T) {
	head := t.TempDir()
	m := InstallMark{From: "https://example.com/a", At: time.Now()}
	require.NoError(t, Install(head, ".", "a", strings.NewReader("whole"), m, 0o644, -1, -1))
	assert.Error(t, Install(head, ".", "a", &failingReader{}, m, 0o644, -1, -1))
	b, err := os.ReadFile(filepath.Join(head, "a"))
	require.NoError(t, err)
	assert.Equal(t, "whole", string(b))
	l, err := os.ReadDir(head)
	require.NoError(t, err)
	assert.Len(t, l, 1)

	assert.Error(t, Install(head, "nope", "a", strings.NewReader("x"), m, 0o644, -1, -1), "a directory not made")
	assert.Error(t, Install(head, ".", "../a", strings.NewReader("x"), m, 0o644, -1, -1), "no file name")
}

// A link at the destination is replaced, not followed out of the volume.
func TestInstallStaysInTheVolume(t *testing.T) {
	head := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(head, "a")))
	m := InstallMark{From: "https://example.com/a", At: time.Now()}
	require.NoError(t, Install(head, ".", "a", strings.NewReader("fetched"), m, 0o644, -1, -1))
	b, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "secret", string(b), "the link is replaced, not followed")
	info, err := os.Lstat(filepath.Join(head, "a"))
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular())
}
