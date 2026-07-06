package sgcp

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

// TestFilesAPIURLMethods tests URL construction for FilesAPI
func TestFilesAPIURLMethods(t *testing.T) {
	defer Setup(t)()

	cfg := GetConfig()
	require.NotNil(t, cfg)

	api := NewFilesAPI(cfg, nil, nil, nil)

	// Test filesystem URL
	assert.Equal(t, "https://127.0.0.1:1215/file/fs/test-uuid",
		api.getFilesystemURL("test-uuid"))

	// Test NFS clients URL
	assert.Equal(t, "https://127.0.0.1:1215/file/fs/test-uuid/client",
		api.getNFSClientsURL("test-uuid"))

	// Test NFS client URL
	assert.Equal(t, "https://127.0.0.1:1215/file/fs/test-uuid/client/client-uuid",
		api.getNFSClientURL("test-uuid", "client-uuid"))

	// Test consistency group URL
	assert.Equal(t, "https://127.0.0.1:1215/file/cg/cg-uuid",
		api.GetConsistencyGroupURL("cg-uuid"))
}

func TestGetFilesystem(t *testing.T) {
	defer Setup(t)()

	cfg := GetConfig()
	require.NotNil(t, cfg)

	// Create a test server
	var calls []string
	var apiCallCount int

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/file/fs/id1" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			auth := r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			t.Logf("GET /file/fs/id1 for auth %s", auth)
			w.WriteHeader(http.StatusOK)
			calls = append(calls, r.URL.Path)
			enc := json.NewEncoder(w)
			enc.Encode(map[string]interface{}{"test": calls})
			apiCallCount++
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	ts := httptest.NewUnstartedServer(handler)

	ln, err := net.Listen("tcp", "127.0.0.1:1215")
	if err != nil {
		t.Fatal(err)
	}

	ts.EnableHTTP2 = true
	ts.Listener = ln
	ts.StartTLS()
	defer ts.Close()

	client := ts.Client()
	log := plog.NewDefaultLogger()
	tk := &TTkBuilder{}
	api := NewFilesAPI(cfg, client, log, tk)

	ctx := context.Background()

	t.Log("expecting all calls hits the api")
	method, url, code, data1, err := api.GetFilesystem(ctx, "id1")
	require.Contains(t, url, "/fs/id1")
	require.Equal(t, "GET", method)
	require.Equal(t, 200, code)
	require.NoError(t, err)
	t.Logf("First request result: %s", string(data1))
	t.Logf("api calls count: %d %v", apiCallCount, calls)
	assert.Equal(t, 1, apiCallCount)

	method, url, code, data1, err = api.GetFilesystem(ctx, "id1")
	require.Contains(t, url, "/fs/id1")
	require.Equal(t, "GET", method)
	require.Equal(t, 200, code)
	require.NoError(t, err)
	t.Logf("Second request result: %s", string(data1))
	t.Logf("api calls count: %d %v", apiCallCount, calls)
	assert.Equal(t, 2, apiCallCount)
}
