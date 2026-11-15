package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// A log read the daemon refuses, as a --grep the journalctl of the node can
// not apply, returns the reason of the problem it answered. The client
// panicked on any answer but 200, closing the body of the response it had
// set to nil (card 304).
func TestALogReadRefusedReturnsTheProblem(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte(`{"title":"Grep unsupported","status":501,"detail":"n1: --grep: unsupported by this journalctl (systemd < 237 or built without PCRE2)"}`))
	}))
	defer srv.Close()
	c, err := New(WithURL(srv.URL), WithInsecureSkipVerify(true))
	require.NoError(t, err)
	grep := "x"
	_, err = c.NewGetLogs("n1").SetGrep(&grep).GetReader()
	require.ErrorContains(t, err, "--grep: unsupported by this journalctl")

	srv2 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv2.Close()
	c, err = New(WithURL(srv2.URL), WithInsecureSkipVerify(true))
	require.NoError(t, err)
	_, err = c.NewGetLogs("n1").GetReader()
	require.ErrorContains(t, err, "500")
}
