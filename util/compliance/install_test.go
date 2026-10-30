package compliance

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func moduleArchive(t *testing.T, entries ...tar.Header) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		content := []byte(e.Uname)
		e.Uname = ""
		e.Size = int64(len(content))
		require.NoError(t, tw.WriteHeader(&e))
		_, err := tw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// regular returns a tar header for a file, content passed through Uname.
func regular(name, content string) tar.Header {
	return tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o755, Uname: content}
}

func newInstallT(t *testing.T) (*T, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "compliance")
	comp := New()
	comp.SetVarDir(dir)
	return comp, dir
}

func TestInstall(t *testing.T) {
	comp, dir := newInstallT(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "S01-old"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "com.opensvc"), 0o755))

	b := moduleArchive(t,
		tar.Header{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": "abc"}},
		tar.Header{Name: "compliance/", Typeflag: tar.TypeDir, Mode: 0o755},
		tar.Header{Name: "compliance/S10-new/", Typeflag: tar.TypeDir, Mode: 0o755},
		regular("compliance/S10-new/main", "#!/bin/sh\n"),
		tar.Header{Name: "compliance/S10-new/link", Typeflag: tar.TypeSymlink, Linkname: "main"},
	)
	require.NoError(t, comp.Install(bytes.NewReader(b)))

	got, err := os.ReadFile(filepath.Join(dir, "S10-new", "main"))
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\n", string(got))
	info, err := os.Stat(filepath.Join(dir, "S10-new", "main"))
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o111, "the modules are executable")
	link, err := os.Readlink(filepath.Join(dir, "S10-new", "link"))
	require.NoError(t, err)
	assert.Equal(t, "main", link)
	assert.NoDirExists(t, filepath.Join(dir, "S01-old"))
	assert.DirExists(t, filepath.Join(dir+".bck", "S01-old"))
	assert.DirExists(t, filepath.Join(dir, "com.opensvc"), "the upstream objects are kept")

	leftovers, err := filepath.Glob(dir + ".tmp-*")
	require.NoError(t, err)
	assert.Empty(t, leftovers)
}

// A refused archive leaves the installed modules alone.
func TestInstallRefused(t *testing.T) {
	cases := map[string][]byte{
		"empty":      moduleArchive(t, tar.Header{Name: "compliance/", Typeflag: tar.TypeDir, Mode: 0o755}),
		"outside":    moduleArchive(t, regular("etc/passwd", "x")),
		"dotdot":     moduleArchive(t, regular("compliance/../../passwd", "x")),
		"abs link":   moduleArchive(t, tar.Header{Name: "compliance/l", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}),
		"up link":    moduleArchive(t, tar.Header{Name: "compliance/d/l", Typeflag: tar.TypeSymlink, Linkname: "../../x"}),
		"device":     moduleArchive(t, tar.Header{Name: "compliance/null", Typeflag: tar.TypeChar}),
		"not a gzip": []byte("not a gzip"),
	}
	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			comp, dir := newInstallT(t)
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "S01-old"), 0o755))
			require.Error(t, comp.Install(bytes.NewReader(b)))
			assert.DirExists(t, filepath.Join(dir, "S01-old"))
			assert.NoDirExists(t, dir+".bck")
			leftovers, err := filepath.Glob(dir + ".tmp-*")
			require.NoError(t, err)
			assert.Empty(t, leftovers)
		})
	}
}
