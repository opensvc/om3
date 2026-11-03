package hbrelay

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A relay whose certificate is signed by an authority the client was built
// without, as a relay of the cluster for a client built before the listener
// installed the cluster authorities, is a trust error: the client is built
// again. Another failure is not.
func TestARelayCertificateNotTrustedIsATrustError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	_, err := (&http.Client{}).Get(srv.URL)
	require.Error(t, err)
	assert.True(t, isTrustError(err), "%s", err)

	_, err = srv.Client().Get(srv.URL)
	require.NoError(t, err)

	assert.False(t, isTrustError(errors.New("connection refused")))
	assert.False(t, isTrustError(nil))
}
