package object

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
)

func setupRegisterNode(t *testing.T, cluster, node string) *Node {
	t.Helper()
	testhelper.Setup(t)
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })
	require.NoError(t, os.WriteFile(rawconfig.ClusterConfigFile(), []byte("[cluster]\nname = c1\n"+cluster), 0600))
	require.NoError(t, os.WriteFile(rawconfig.NodeConfigFile(), []byte(node), 0600))
	n, err := NewNode()
	require.NoError(t, err)
	return n
}

// TestNodeClusterID pins that the cluster id sent on the registration is the
// one of the cluster config.
func TestNodeClusterID(t *testing.T) {
	n := setupRegisterNode(t, "id = 0b3b5f9c-1111-4f00-9a00-000000000001\n", "")
	assert.Equal(t, "0b3b5f9c-1111-4f00-9a00-000000000001", n.clusterID())
}

// TestRegisterJoiningNode pins that the node registers the joining node on
// the collector authenticated by its own registration id, with the joining
// node previous registration id, and returns the new one.
func TestRegisterJoiningNode(t *testing.T) {
	var (
		gotUser, gotPassword string
		gotBody              registerReq
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != oc3path.ServerRegister || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		gotUser, gotPassword, _ = r.BasicAuth()
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(registerResOK{UUID: "uuid-new", Info: "node is enrolled"})
	}))
	defer srv.Close()

	n := setupRegisterNode(t, "[collector]\nserver = "+srv.URL+"\n", "[node]\nuuid = uuid-enroller\n")
	nodeUUID, err := n.RegisterJoiningNode(context.Background(), "node3", "uuid-old")
	require.NoError(t, err)
	assert.Equal(t, "uuid-new", nodeUUID)
	assert.Equal(t, hostname.Hostname(), gotUser, "authenticated as the enroller")
	assert.Equal(t, "uuid-enroller", gotPassword)
	assert.Equal(t, registerReq{Nodename: "node3", UUID: "uuid-old"}, gotBody)
}

// TestRegisterJoiningNodeRefused pins that a collector refusal is an error
// carrying the collector reason.
func TestRegisterJoiningNodeRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"title":"Forbidden","detail":"the joining node is not registered","status":403}`))
	}))
	defer srv.Close()

	n := setupRegisterNode(t, "[collector]\nserver = "+srv.URL+"\n", "[node]\nuuid = uuid-enroller\n")
	_, err := n.RegisterJoiningNode(context.Background(), "node3", "uuid-old")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

// TestRegisterJoiningNodeUnregistered pins that a node without registration
// id can't register a joining node.
func TestRegisterJoiningNodeUnregistered(t *testing.T) {
	n := setupRegisterNode(t, "[collector]\nserver = https://oc3/server\n", "")
	_, err := n.RegisterJoiningNode(context.Background(), "node3", "uuid-old")
	assert.ErrorIs(t, err, collector.ErrUnregistered)
}
