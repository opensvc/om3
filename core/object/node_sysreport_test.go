package object

import (
	"io"
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

// A sysreport is posted to the oc3 feeder as a multipart form: the archive
// in the file part, each deleted file in a deleted part, and the full flag.
func TestSendSysreport(t *testing.T) {
	var (
		gotFile    string
		gotDeleted []string
		gotFull    string
		gotPath    string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if f, _, err := r.FormFile("file"); err == nil {
			b, _ := io.ReadAll(f)
			gotFile = string(b)
		}
		gotDeleted = r.MultipartForm.Value["deleted"]
		gotFull = r.FormValue("full")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	testhelper.Setup(t)
	conf := "[node]\nuuid = 00000000-0000-0000-0000-000000000001\ncollector_feeder = " + server.URL + "\n"
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte(conf), 0600))
	n, err := NewNode()
	require.NoError(t, err)

	archive := filepath.Join(t.TempDir(), "sysreport.1.tar")
	require.NoError(t, os.WriteFile(archive, []byte("tar content"), 0600))
	require.NoError(t, n.SendSysreport(archive, []string{"/etc/a", "/etc/b"}, true))
	assert.Equal(t, "/api/node/sysreport", gotPath)
	assert.Equal(t, "tar content", gotFile)
	assert.Equal(t, []string{"/etc/a", "/etc/b"}, gotDeleted)
	assert.Equal(t, "true", gotFull)

	// A report of deletions only has no file part.
	gotFile = ""
	require.NoError(t, n.SendSysreport("", []string{"/etc/c"}, false))
	assert.Equal(t, "", gotFile)
	assert.Equal(t, []string{"/etc/c"}, gotDeleted)
	assert.Equal(t, "false", gotFull)
}
