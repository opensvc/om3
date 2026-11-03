package collector

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The jsonrpc client verifies the certificate of the collector, as its rest
// clients do, unless node.dbinsecure says the collector is a test one.
func TestTheJsonrpcClientVerifiesTheCollectorCertificate(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":0,"result":"ok"}`))
	}))
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	c, err := newClient(u, "secret", false)
	require.NoError(t, err)
	_, err = c.Call("ping")
	assert.ErrorContains(t, err, "certificate", "an untrusted collector is refused")

	c, err = newClient(u, "secret", true)
	require.NoError(t, err)
	_, err = c.Call("ping")
	assert.NoError(t, err, "node.dbinsecure accepts it")
}
