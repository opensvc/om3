package object

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
)

func TestUpdateCompliance(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "compliance/S10-a/main", Typeflag: tar.TypeReg, Mode: 0o755, Size: 2}))
	_, err := tw.Write([]byte("ok"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	var gotPath string
	var gotAuth bool
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _, gotAuth = r.BasicAuth()
		w.Header().Set("X-Compliance-Modules-Commit", "abc123")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write(buf.Bytes())
		}
	}))
	defer server.Close()

	testhelper.Setup(t)
	conf := "[node]\nuuid = 00000000-0000-0000-0000-000000000001\n[collector]\nserver = " + server.URL + "\n"
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte(conf), 0600))
	n, err := NewNode()
	require.NoError(t, err)

	commit, err := n.UpdateCompliance()
	require.NoError(t, err)
	assert.Equal(t, "abc123", commit)
	assert.Equal(t, "/api/compliance/modules_repo/current", gotPath)
	assert.True(t, gotAuth, "the node authenticates")
	got, err := os.ReadFile(filepath.Join(rawconfig.Paths.Var, "compliance", "S10-a", "main"))
	require.NoError(t, err)
	assert.Equal(t, "ok", string(got))

	// Nothing to install leaves the modules in place.
	status = http.StatusNotFound
	_, err = n.UpdateCompliance()
	require.ErrorContains(t, err, "no compliance module")
	assert.FileExists(t, filepath.Join(rawconfig.Paths.Var, "compliance", "S10-a", "main"))
}
