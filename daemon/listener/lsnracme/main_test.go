package lsnracme

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
)

func TestTheListenerListensOnlyWhenAPortIsSet(t *testing.T) {
	cfg := &cluster.Config{}
	assert.Equal(t, "", configuredAddr(cfg))
	assert.Equal(t, "", configuredAddr(nil))
	cfg.Listener.ACMEPort = 80
	assert.Equal(t, ":80", configuredAddr(cfg))
	cfg.Listener.Addr = "fd00::1"
	assert.Equal(t, "[fd00::1]:80", configuredAddr(cfg))
}

// Only a token of a sec of listener.tls_secs is answered: no file, no other
// key, no other path.
func TestOnlyAChallengeTokenIsAnswered(t *testing.T) {
	cluster.ConfigData.Set(&cluster.Config{})
	lsnr := New()
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/"},
		{http.MethodGet, "/api/node"},
		{http.MethodGet, "/.well-known/acme-challenge/"},
		{http.MethodGet, "/.well-known/acme-challenge/unknown"},
		{http.MethodGet, "/.well-known/acme-challenge/../private_key"},
		{http.MethodPost, "/.well-known/acme-challenge/token"},
	} {
		w := httptest.NewRecorder()
		lsnr.serve(w, httptest.NewRequest(c.method, c.path, nil))
		assert.Equal(t, http.StatusNotFound, w.Code, "%s %s", c.method, c.path)
	}
}

// A port taken when the listener starts is not taken for listened on: the
// next try listens once it is free.
func TestAPortNotListenedOnIsTriedAgain(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := taken.Addr().String()
	lsnr := New()
	lsnr.apply(addr)
	assert.Nil(t, lsnr.server)
	require.NoError(t, taken.Close())
	lsnr.apply(addr)
	assert.NotNil(t, lsnr.server)
	lsnr.apply("")
	assert.Nil(t, lsnr.server)
}

func TestTheRequestsBeyondTheBoundAreAnsweredBusy(t *testing.T) {
	cluster.ConfigData.Set(&cluster.Config{})
	lsnr := New()
	for i := 0; i < maxConcurrent; i++ {
		lsnr.busy <- struct{}{}
	}
	w := httptest.NewRecorder()
	lsnr.serve(w, httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/token", nil))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
