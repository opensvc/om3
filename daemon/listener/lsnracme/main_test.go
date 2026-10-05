package lsnracme

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

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
