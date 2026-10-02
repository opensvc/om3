package reqh2

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/client/tokencache"
	"github.com/opensvc/om3/v3/util/tokenstore"
)

// A request refused for an expired openid access token is retried with a
// token refreshed at the issuer, not at the cluster, and the refreshed token
// is saved for the contexts sharing it.
func TestRefreshTransportRefreshesOpenIDAtTheIssuer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OSVC_CONTEXT", "c1")
	homedir.Reset()
	t.Cleanup(homedir.Reset)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "opensvc"), 0o700))

	issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "refresh1" || r.Form.Get("client_id") != "om3" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access2", "token_type": "Bearer", "expires_in": 300})
	}))
	defer issuer.Close()
	clusterRefreshed := false
	cluster := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/refresh" {
			clusterRefreshed = true
		}
		if r.Header.Get("Authorization") != "Bearer access2" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer cluster.Close()

	ref := &tokencache.OpenID{Issuer: "https://idp/app/", ClientID: "om3", TokenEndpoint: issuer.URL, Store: tokenstore.File}
	tokens := tokencache.Entry{AccessToken: "access1", AccessTokenExpire: time.Now().Add(-time.Minute), RefreshToken: "refresh1", OpenID: ref}
	require.NoError(t, tokencache.Save("c1", tokens))
	require.NoError(t, tokencache.Save("c2", tokens))

	tr := &RefreshTransport{Base: http.DefaultTransport, baseURL: cluster.URL, tokens: tokens}
	req, err := http.NewRequest(http.MethodGet, cluster.URL+"/api/node", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer access1")
	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.False(t, clusterRefreshed, "the cluster does not refresh a token it did not issue")

	shared, err := tokencache.Load("c2")
	require.NoError(t, err)
	assert.Equal(t, "access2", shared.AccessToken, "the other context of the issuer has the refreshed token")
	assert.Equal(t, "refresh1", shared.RefreshToken)
}

// A context never logged in, whose cluster trusts the issuer and client
// another context logged in at, uses its tokens with no challenge, and keeps
// them as its own.
func TestRefreshTransportAdoptsTheSharedOpenIDToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SSH_AUTH_SOCK", "")
	homedir.Reset()
	t.Cleanup(homedir.Reset)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "opensvc"), 0o700))

	issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access2", "token_type": "Bearer", "expires_in": 300})
	}))
	defer issuer.Close()
	cluster := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/info" {
			_, _ = w.Write([]byte(`{"methods":["openid"],"openid":{"issuer":"https://idp/app/","client_id":"om3"}}`))
			return
		}
		if r.Header.Get("Authorization") != "Bearer access2" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer cluster.Close()

	ref := &tokencache.OpenID{Issuer: "https://idp/app/", ClientID: "om3", TokenEndpoint: issuer.URL, Store: tokenstore.File}
	t.Setenv("OSVC_CONTEXT", "c1")
	require.NoError(t, tokencache.Save("c1", tokencache.Entry{AccessToken: "access1", RefreshToken: "refresh1", OpenID: ref}))

	t.Setenv("OSVC_CONTEXT", "c2")
	tr := &RefreshTransport{Base: http.DefaultTransport, baseURL: cluster.URL}
	req, err := http.NewRequest(http.MethodGet, cluster.URL+"/api/node", nil)
	require.NoError(t, err)
	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	own, err := tokencache.Load("c2")
	require.NoError(t, err)
	require.NotNil(t, own)
	assert.Equal(t, "access2", own.AccessToken, "c2 keeps the adopted tokens as its own")
}
