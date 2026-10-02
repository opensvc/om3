package oxcmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/client/tokencache"
	"github.com/opensvc/om3/v3/util/oidc"
	"github.com/opensvc/om3/v3/util/tokenstore"
)

// openIDOffer returns the openid issuer and client the cluster of the
// current context trusts, empty when it offers no openid login. The cluster
// answers it without authentication.
func openIDOffer() (string, string, error) {
	c, err := client.New()
	if err != nil {
		return "", "", err
	}
	resp, err := c.GetAuthInfoWithResponse(context.Background())
	if err != nil {
		return "", "", fmt.Errorf("ask the cluster how to log in: %w", err)
	}
	if resp.JSON200 == nil {
		return "", "", fmt.Errorf("ask the cluster how to log in: %s", resp.Status())
	}
	if o := resp.JSON200.Openid; o != nil && o.Issuer != "" {
		return o.Issuer, o.ClientId, nil
	}
	return "", "", nil
}

// loginOpenID logs the context in at the openid issuer its cluster trusts.
//
// The tokens of an issuer and client are shared by the contexts logging in
// there: a context whose issuer and client another context logged in at is
// logged in with their refresh token, with no challenge. A challenge opens
// the browser of this machine, or asks for a device code where there is none.
func (t *CmdContextLogin) loginOpenID(cmd *cobra.Command, issuer, clientID string) error {
	if cmd.Flag("duration").Changed || cmd.Flag("refresh-duration").Changed {
		return fmt.Errorf("--duration and --refresh-duration apply to a password login: the openid issuer sets the token durations")
	}
	backend, err := tokenstore.ParseBackend(t.Cache)
	if err != nil {
		return err
	}
	ctx := context.Background()
	provider, err := oidc.Discover(ctx, issuer)
	if err != nil {
		return err
	}

	var (
		tok      *oidc.Token
		existing = tokencache.FindOpenID(issuer, clientID)
	)
	if existing != nil && existing.HasValidRefresh(time.Now()) {
		if backend == "" {
			backend = existing.OpenID.Store
		}
		if refreshed, err := oidc.Refresh(ctx, provider.TokenEndpoint, clientID, existing.RefreshToken); err == nil {
			tok = refreshed
			fmt.Printf("Logged in with the token cached for %s, no challenge needed.\n", issuer)
		} else {
			fmt.Fprintf(os.Stderr, "The token cached for %s is refused, log in again: %s\n", issuer, err)
		}
	}
	if tok == nil {
		if tok, err = t.challenge(ctx, provider, clientID); err != nil {
			return err
		}
	}

	store, err := tokencache.OpenStore(backend)
	if err != nil {
		return err
	}
	entry := tokencache.Entry{
		AccessToken:        tok.AccessToken,
		AccessTokenExpire:  tok.AccessExpiry,
		RefreshToken:       tok.RefreshToken,
		RefreshTokenExpire: tok.RefreshExpiry,
		OpenID: &tokencache.OpenID{
			Issuer:        issuer,
			ClientID:      clientID,
			TokenEndpoint: provider.TokenEndpoint,
			Store:         store.Backend(),
		},
	}
	if err := tokencache.Save(t.Context, entry); err != nil {
		return err
	}
	if existing != nil && existing.OpenID.Store != store.Backend() {
		// The issuer may have rotated the refresh token the other
		// contexts read from the other store: they read the new one.
		other := entry
		ref := *existing.OpenID
		other.OpenID = &ref
		if err := tokencache.SaveShared(other); err != nil {
			fmt.Fprintf(os.Stderr, "Update the token the other contexts of %s read in the %s store: %s\n", issuer, ref.Store, err)
		}
	}
	if entry.RefreshToken == "" {
		fmt.Fprintf(os.Stderr, "The issuer gave no refresh token: log in again when the access token expires, at %s. Grant the offline_access scope to the client to stay logged in.\n", entry.AccessTokenExpire.Local().Format(time.RFC3339))
	}
	user := tokenUser(entry.AccessToken)
	if user != "" {
		user = " as " + user
	}
	fmt.Printf("Login successful%s, tokens kept in the %s store. Switch to this context with :\nexport OSVC_CONTEXT=%s\n", user, store.Backend(), t.Context)
	return nil
}

// challenge logs the user in at the issuer: in the browser of this machine,
// or with a device code from a browser anywhere when this machine has none
// or --device asks for it.
func (t *CmdContextLogin) challenge(ctx context.Context, provider *oidc.Provider, clientID string) (*oidc.Token, error) {
	if t.Device || !oidc.HasBrowser() {
		if !t.Device {
			fmt.Println("No browser to open on this machine: log in from a browser anywhere with a device code.")
		}
		return provider.LoginDevice(ctx, clientID, func(uri, code, complete string) {
			fmt.Printf("Open %s and enter the code %s", uri, code)
			if complete != "" {
				fmt.Printf(", or open %s", complete)
			}
			fmt.Println("\nWaiting for the login ...")
		})
	}
	return provider.LoginBrowser(ctx, clientID, func(u string) error {
		fmt.Printf("Log in at %s in the browser.\n", provider.Issuer)
		if err := oidc.OpenBrowser(u); err != nil {
			fmt.Printf("Open this url in a browser of this machine:\n%s\n", u)
		}
		fmt.Println("Waiting for the login ...")
		return nil
	})
}

// tokenUser returns who an access token says it is about, to tell the user
// who they logged in as. The token is not verified here: the cluster
// verifies it, and this is only shown.
func tokenUser(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		PreferredUsername string `json:"preferred_username"`
		Email             string `json:"email"`
		Subject           string `json:"sub"`
	}
	if err := json.Unmarshal(b, &claims); err != nil {
		return ""
	}
	for _, s := range []string{claims.PreferredUsername, claims.Email, claims.Subject} {
		if s != "" {
			return s
		}
	}
	return ""
}
