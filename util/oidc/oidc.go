// Package oidc logs a user in at an OpenID Connect provider, from a command
// line program, and refreshes the tokens it obtained.
//
// The login is the authorization code grant with PKCE, its redirect landing on
// a listener of the loopback address the browser of the user reaches, as RFC
// 8252 says a native application does. Where no browser runs on the machine
// of the program, as over ssh, the device authorization grant of RFC 8628
// logs the user in from a browser anywhere.
//
// The client is a public one: it holds no secret, its PKCE verifier proving
// the code is redeemed by who asked for it.
package oidc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

type (
	// Provider is what the discovery document of an issuer says of it.
	Provider struct {
		Issuer                      string   `json:"issuer"`
		AuthorizationEndpoint       string   `json:"authorization_endpoint"`
		TokenEndpoint               string   `json:"token_endpoint"`
		DeviceAuthorizationEndpoint string   `json:"device_authorization_endpoint,omitempty"`
		ScopesSupported             []string `json:"scopes_supported,omitempty"`
	}

	// Token is what a login or a refresh obtained.
	Token struct {
		AccessToken  string
		AccessExpiry time.Time

		// RefreshToken is empty when the provider gave none.
		RefreshToken string

		// RefreshExpiry is zero when the provider does not say when the
		// refresh token expires, as most do not: it is then tried until it
		// is refused.
		RefreshExpiry time.Time
	}
)

const (
	// CallbackPath is the path of the redirect the loopback listener
	// serves. The redirect uri the provider allows is
	// http://127.0.0.1:<any port>/callback.
	CallbackPath = "/callback"

	// LoginTimeout bounds the time the user is given to log in.
	LoginTimeout = 5 * time.Minute

	discoveryTimeout = 10 * time.Second
)

// wantedScopes are the scopes asked when the provider supports them: who the
// user is, a refresh token, and the grants of om3.
var wantedScopes = []string{
	"openid",
	"profile",
	"email",
	"offline_access",
	"opensvc:om3",
	"opensvc:om3:root",
	"opensvc:om3:operator",
	"opensvc:om3:guest",
}

// ErrNoRefreshToken is returned on a refresh asked with no refresh token.
var ErrNoRefreshToken = errors.New("no refresh token")

// Discover reads the discovery document of an issuer.
//
// The issuer is named by the server the program connects to, so it is not
// followed through redirects, which could lead the request somewhere the
// server did not name, and the document must name the issuer it was asked
// for.
func Discover(ctx context.Context, issuer string) (*Provider, error) {
	ctx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()
	u := strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return fmt.Errorf("the discovery document of %s redirects, which is not followed", issuer)
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", issuer, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discover %s: %s", issuer, resp.Status)
	}
	var p Provider
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("discover %s: %w", issuer, err)
	}
	if strings.TrimSuffix(p.Issuer, "/") != strings.TrimSuffix(issuer, "/") {
		return nil, fmt.Errorf("discover %s: the document is the one of issuer %s", issuer, p.Issuer)
	}
	if p.AuthorizationEndpoint == "" || p.TokenEndpoint == "" {
		return nil, fmt.Errorf("discover %s: no authorization or token endpoint", issuer)
	}
	return &p, nil
}

// Scopes returns the scopes to ask: the wanted ones the provider supports,
// all of them when it does not say what it supports.
func (p *Provider) Scopes() []string {
	if len(p.ScopesSupported) == 0 {
		return slices.Clone(wantedScopes)
	}
	l := make([]string, 0, len(wantedScopes))
	for _, scope := range wantedScopes {
		if slices.Contains(p.ScopesSupported, scope) {
			l = append(l, scope)
		}
	}
	return l
}

func (p *Provider) config(clientID, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID: clientID,
		Endpoint: oauth2.Endpoint{
			AuthURL:       p.AuthorizationEndpoint,
			TokenURL:      p.TokenEndpoint,
			DeviceAuthURL: p.DeviceAuthorizationEndpoint,
			AuthStyle:     oauth2.AuthStyleInParams,
		},
		RedirectURL: redirectURL,
		Scopes:      p.Scopes(),
	}
}

// LoginBrowser logs the user in with the authorization code grant and PKCE.
//
// It listens on a port of 127.0.0.1, and hands the authorization url to open,
// which opens the browser of the user on it, or tells them to. The browser
// lands back on the listener with the code, which is redeemed with the
// verifier only this call knows.
func (p *Provider) LoginBrowser(ctx context.Context, clientID string, open func(url string) error) (*Token, error) {
	ctx, cancel := context.WithTimeout(ctx, LoginTimeout)
	defer cancel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for the login redirect: %w", err)
	}
	defer func() { _ = ln.Close() }()
	redirectURL := fmt.Sprintf("http://%s%s", ln.Addr().String(), CallbackPath)
	cfg := p.config(clientID, redirectURL)
	state, err := randomString()
	if err != nil {
		return nil, err
	}
	verifier := oauth2.GenerateVerifier()

	type result struct {
		code string
		err  error
	}
	resultC := make(chan result, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(CallbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var res result
		switch {
		case q.Get("state") != state:
			// Not the answer to this login: a page of another
			// origin may make the browser request the listener.
			http.Error(w, "unexpected state", http.StatusBadRequest)
			return
		case q.Get("error") != "":
			res.err = fmt.Errorf("the provider refused the login: %s %s", q.Get("error"), q.Get("error_description"))
		case q.Get("code") == "":
			res.err = errors.New("the provider redirected with no code")
		default:
			res.code = q.Get("code")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if res.err != nil {
			_, _ = fmt.Fprintf(w, "<html><body><p>Login failed: %s</p></body></html>", html.EscapeString(res.err.Error()))
		} else {
			_, _ = fmt.Fprint(w, "<html><body><p>Logged in. You can close this window.</p></body></html>")
		}
		select {
		case resultC <- res:
		default:
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	authURL := cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
	if err := open(authURL); err != nil {
		return nil, err
	}
	var res result
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("no login within %s: %w", LoginTimeout, ctx.Err())
	case res = <-resultC:
	}
	if res.err != nil {
		return nil, res.err
	}
	tok, err := cfg.Exchange(ctx, res.code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("redeem the login code: %w", err)
	}
	return newToken(tok), nil
}

// LoginDevice logs the user in with the device authorization grant: prompt
// is told the url to open and the code to enter there, from any browser, and
// the call returns once the user approved.
func (p *Provider) LoginDevice(ctx context.Context, clientID string, prompt func(verificationURI, userCode, verificationURIComplete string)) (*Token, error) {
	if p.DeviceAuthorizationEndpoint == "" {
		return nil, fmt.Errorf("the issuer %s offers no device authorization", p.Issuer)
	}
	ctx, cancel := context.WithTimeout(ctx, LoginTimeout)
	defer cancel()
	cfg := p.config(clientID, "")
	da, err := cfg.DeviceAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("start the device login: %w", err)
	}
	validity := time.Until(da.Expiry).Round(time.Second)
	prompt(da.VerificationURI, da.UserCode, da.VerificationURIComplete)
	tok, err := cfg.DeviceAccessToken(ctx, da)
	switch {
	case errors.Is(err, context.DeadlineExceeded) && !da.Expiry.IsZero() && !time.Now().Before(da.Expiry):
		return nil, fmt.Errorf("the code %s expired before the login was approved: the issuer gives %s to approve a code", da.UserCode, validity)
	case err != nil:
		return nil, fmt.Errorf("device login: %w", err)
	}
	return newToken(tok), nil
}

// Refresh obtains a new access token from a refresh token, at the token
// endpoint of the provider. The provider may rotate the refresh token, and
// the new one is returned then.
func Refresh(ctx context.Context, tokenEndpoint, clientID, refreshToken string) (*Token, error) {
	if refreshToken == "" {
		return nil, ErrNoRefreshToken
	}
	cfg := &oauth2.Config{
		ClientID: clientID,
		Endpoint: oauth2.Endpoint{TokenURL: tokenEndpoint, AuthStyle: oauth2.AuthStyleInParams},
	}
	expired := &oauth2.Token{RefreshToken: refreshToken, Expiry: time.Unix(1, 0)}
	tok, err := cfg.TokenSource(ctx, expired).Token()
	if err != nil {
		return nil, fmt.Errorf("refresh the token: %w", err)
	}
	t := newToken(tok)
	if t.RefreshToken == "" {
		t.RefreshToken = refreshToken
	}
	return t, nil
}

func newToken(tok *oauth2.Token) *Token {
	t := &Token{
		AccessToken:  tok.AccessToken,
		AccessExpiry: tok.Expiry,
		RefreshToken: tok.RefreshToken,
	}
	switch v := tok.Extra("refresh_expires_in").(type) {
	case float64:
		if v > 0 {
			t.RefreshExpiry = time.Now().Add(time.Duration(v) * time.Second)
		}
	}
	return t
}

func randomString() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
