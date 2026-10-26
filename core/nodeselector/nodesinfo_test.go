package nodeselector

import (
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodesinfo"
	"github.com/opensvc/om3/v3/testhelper"
)

// A caller that can not read the nodes info cache file, a user other than
// root or a host where no daemon ever wrote it, expands a selector all the
// same, asking the daemon api for the nodes info.
func TestExpandAsksTheAPIWhenTheCacheFileIsNotReadable(t *testing.T) {
	testhelper.Setup(t)
	_, err := nodesinfo.Load()
	require.Error(t, err, "the cache file of an empty root is missing")

	sock := filepath.Join(t.TempDir(), "http.sock")
	l, err := net.Listen("unix", sock)
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/node/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(nodesinfo.M{"n2": nodesinfo.T{}, "n1": nodesinfo.T{}})
	})
	srv := &http.Server{Handler: h2c.NewHandler(mux, &http2.Server{})}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })

	c, err := client.New(client.WithURL("http://" + sock))
	require.NoError(t, err)
	nodes, err := New("*", WithClient(c)).Expand()
	require.NoError(t, err)
	assert.Equal(t, []string{"n1", "n2"}, nodes)
}

// With no client to ask, the error reading the cache file is the answer.
func TestExpandWithNoClientFailsWhenTheCacheFileIsNotReadable(t *testing.T) {
	testhelper.Setup(t)
	_, err := New("*").Expand()
	assert.Error(t, err)
}
