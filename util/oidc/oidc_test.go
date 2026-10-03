package oidc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeIssuer is a provider granting the authorization code with PKCE, the
// device code, and the refresh token, to the public client "om3".
type fakeIssuer struct {
	*httptest.Server
	mu         sync.Mutex
	challenges map[string]string // code -> challenge
	redirects  map[string]string // code -> redirect uri
	devices    map[string]bool   // device code -> approved
	refreshes  int
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	f := &fakeIssuer{challenges: map[string]string{}, redirects: map[string]string{}, devices: map[string]bool{}}
	mux := http.NewServeMux()
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Provider{
			Issuer:                      f.URL,
			AuthorizationEndpoint:       f.URL + "/authorize",
			TokenEndpoint:               f.URL + "/token",
			DeviceAuthorizationEndpoint: f.URL + "/device",
			ScopesSupported:             []string{"openid", "offline_access", "opensvc:om3", "unrelated"},
		})
	})
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("client_id") != "om3" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
			http.Error(w, "bad authorize request", http.StatusBadRequest)
			return
		}
		redirect := q.Get("redirect_uri")
		if !strings.HasPrefix(redirect, "http://127.0.0.1:") || !strings.HasSuffix(redirect, CallbackPath) {
			http.Error(w, "redirect uri not allowed", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.challenges["code1"] = q.Get("code_challenge")
		f.redirects["code1"] = redirect
		f.mu.Unlock()
		http.Redirect(w, r, redirect+"?code=code1&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
	})
	mux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		f.devices["dev1"] = false
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dev1", "user_code": "ABCD-EFGH",
			"verification_uri": f.URL + "/activate", "expires_in": 60, "interval": 1,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		deny := func(e string) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": e})
		}
		if r.Form.Get("client_id") != "om3" {
			deny("invalid_client")
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if f.challenges[r.Form.Get("code")] != base64.RawURLEncoding.EncodeToString(sum[:]) {
				deny("invalid_grant")
				return
			}
			if f.redirects[r.Form.Get("code")] != r.Form.Get("redirect_uri") {
				deny("invalid_grant")
				return
			}
		case "urn:ietf:params:oauth:grant-type:device_code":
			if !f.devices[r.Form.Get("device_code")] {
				deny("authorization_pending")
				return
			}
		case "refresh_token":
			if r.Form.Get("refresh_token") != "refresh1" {
				deny("invalid_grant")
				return
			}
			f.refreshes++
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access2", "token_type": "Bearer", "expires_in": 300})
			return
		default:
			deny("unsupported_grant_type")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access1", "token_type": "Bearer", "expires_in": 300,
			"refresh_token": "refresh1", "refresh_expires_in": 3600,
		})
	})
	return f
}

// The browser login redeems the code the provider redirects the browser to
// the loopback listener with, with the PKCE verifier matching its challenge.
func TestLoginBrowser(t *testing.T) {
	f := newFakeIssuer(t)
	p, err := Discover(context.Background(), f.URL)
	require.NoError(t, err)
	assert.Equal(t, []string{"openid", "offline_access", "opensvc:om3"}, p.Scopes(), "the wanted scopes the provider supports")

	// The browser: it follows the redirect of the provider to the
	// listener of the login.
	browser := func(u string) error {
		go func() {
			resp, err := http.Get(u)
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
		return nil
	}
	tok, err := p.LoginBrowser(context.Background(), "om3", browser)
	require.NoError(t, err)
	assert.Equal(t, "access1", tok.AccessToken)
	assert.Equal(t, "refresh1", tok.RefreshToken)
	assert.WithinDuration(t, time.Now().Add(time.Hour), tok.RefreshExpiry, time.Minute)
}

// A redirect carrying the state of another login is refused, and the login
// goes on waiting for its own.
func TestLoginBrowserRefusesAnotherState(t *testing.T) {
	f := newFakeIssuer(t)
	p, err := Discover(context.Background(), f.URL)
	require.NoError(t, err)
	browser := func(u string) error {
		go func() {
			parsed, _ := url.Parse(u)
			redirect := parsed.Query().Get("redirect_uri")
			resp, err := http.Get(redirect + "?code=forged&state=other")
			if err == nil {
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
				_ = resp.Body.Close()
			}
			resp, err = http.Get(u)
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
		return nil
	}
	tok, err := p.LoginBrowser(context.Background(), "om3", browser)
	require.NoError(t, err)
	assert.Equal(t, "access1", tok.AccessToken)
}

// The device login returns once the user approved the code.
func TestLoginDevice(t *testing.T) {
	f := newFakeIssuer(t)
	p, err := Discover(context.Background(), f.URL)
	require.NoError(t, err)
	var shown string
	tok, err := p.LoginDevice(context.Background(), "om3", func(uri, code, _ string) {
		shown = uri + " " + code
		go func() {
			time.Sleep(1500 * time.Millisecond)
			f.mu.Lock()
			f.devices["dev1"] = true
			f.mu.Unlock()
		}()
	})
	require.NoError(t, err)
	assert.Equal(t, f.URL+"/activate ABCD-EFGH", shown)
	assert.Equal(t, "access1", tok.AccessToken)
}

// A refresh keeps the refresh token the provider did not rotate.
func TestRefresh(t *testing.T) {
	f := newFakeIssuer(t)
	tok, err := Refresh(context.Background(), f.URL+"/token", "om3", "refresh1")
	require.NoError(t, err)
	assert.Equal(t, "access2", tok.AccessToken)
	assert.Equal(t, "refresh1", tok.RefreshToken)
	assert.Equal(t, 1, f.refreshes)

	_, err = Refresh(context.Background(), f.URL+"/token", "om3", "revoked")
	assert.Error(t, err)
	_, err = Refresh(context.Background(), f.URL+"/token", "om3", "")
	assert.ErrorIs(t, err, ErrNoRefreshToken)
}

// A discovery document naming another issuer, or redirecting, is refused.
func TestDiscoverRefusals(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Provider{Issuer: "https://elsewhere", AuthorizationEndpoint: "a", TokenEndpoint: "t"})
	}))
	defer other.Close()
	_, err := Discover(context.Background(), other.URL)
	assert.ErrorContains(t, err, "issuer https://elsewhere")

	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/.well-known/openid-configuration", http.StatusFound)
	}))
	defer redirecting.Close()
	_, err = Discover(context.Background(), redirecting.URL)
	assert.ErrorContains(t, err, "redirects")
}

// A device code nobody approves in time says it expired.
func TestLoginDeviceExpired(t *testing.T) {
	f := newFakeIssuer(t)
	p, err := Discover(context.Background(), f.URL)
	require.NoError(t, err)
	f.Config.Handler.(*http.ServeMux).HandleFunc("/device-short", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dev1", "user_code": "WXYZ",
			"verification_uri": f.URL + "/activate", "expires_in": 2, "interval": 1,
		})
	})
	p.DeviceAuthorizationEndpoint = f.URL + "/device-short"
	_, err = p.LoginDevice(context.Background(), "om3", func(string, string, string) {})
	assert.ErrorContains(t, err, "the code WXYZ expired before the login was approved")
}
