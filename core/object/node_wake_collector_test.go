package object

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
)

func TestNodeWakeByCollector(t *testing.T) {
	env := testhelper.Setup(t)
	t.Cleanup(hostname.Impersonate("k1"))
	token := "4b4c1c8e-0000-4000-8000-5ec2e7000000"

	var gotPath, gotQuery, gotAuth, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery, gotAuth = r.Method, r.URL.Path, r.URL.Query().Get("nodename"), r.Header.Get("Authorization")
		switch gotQuery {
		case "k2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"action_id":630,"hypervisor":"hyper","nodename":"k2","path":"lab/svc/kvm","queued":true,"rid":"container#2"}`))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"text":"no node ` + gotQuery + ` in the cluster of k1"}`))
		}
	}))
	t.Cleanup(srv.Close)
	require.NoError(t, os.WriteFile(filepath.Join(env.Root, "etc", "cluster.conf"),
		[]byte("[cluster]\nname = kvm\nnodes = k1 k2\n\n[collector]\nserver = "+srv.URL+"\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(env.Root, "etc", "node.conf"),
		[]byte("[node]\nuuid = "+token+"\n"), 0o600))

	n, err := NewNode()
	require.NoError(t, err)

	ans, err := n.wakeByCollector("k2")
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/api/agent/wake", gotPath)
	assert.Equal(t, "k2", gotQuery)
	assert.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("k1:"+token)), gotAuth)
	assert.Equal(t, wakeCollectorAnswer{ActionID: 630, Hypervisor: "hyper", Nodename: "k2", Path: "lab/svc/kvm", RID: "container#2", Queued: true}, ans)

	_, err = n.wakeByCollector("nope")
	assert.ErrorContains(t, err, "no node nope in the cluster of k1")
}
