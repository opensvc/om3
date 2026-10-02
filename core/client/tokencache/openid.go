package tokencache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"

	"github.com/opensvc/om3/v3/core/clientcontext"
	"github.com/opensvc/om3/v3/util/tokenstore"
)

type (
	// OpenID names the tokens an openid issuer gave for a client, and where
	// they are kept.
	OpenID struct {
		Issuer        string             `json:"issuer"`
		ClientID      string             `json:"client_id"`
		TokenEndpoint string             `json:"token_endpoint"`
		Store         tokenstore.Backend `json:"store"`
	}

	// openIDTokens is what the token store keeps of an openid login.
	openIDTokens struct {
		Issuer             string    `json:"issuer"`
		ClientID           string    `json:"client_id"`
		TokenEndpoint      string    `json:"token_endpoint"`
		AccessToken        string    `json:"access_token"`
		AccessTokenExpire  time.Time `json:"access_expired_at"`
		RefreshToken       string    `json:"refresh_token"`
		RefreshTokenExpire time.Time `json:"refresh_expired_at,omitzero"`
	}
)

// Name is the name of the entry of the token store the tokens are kept in,
// the same for every context of the issuer and client.
func (t OpenID) Name() string {
	sum := sha256.Sum256([]byte(t.Issuer + "\x00" + t.ClientID))
	return "openid-" + hex.EncodeToString(sum[:12])
}

// HasValidRefresh says whether the refresh token of an entry can be tried:
// one whose expiry the issuer did not tell is tried until it is refused.
func (t Entry) HasValidRefresh(now time.Time) bool {
	if t.RefreshToken == "" {
		return false
	}
	return t.RefreshTokenExpire.IsZero() || now.Before(t.RefreshTokenExpire)
}

func storeDir() (string, error) {
	return homedir.Expand(clientcontext.ConfigFolder)
}

func openStore(backend tokenstore.Backend) (tokenstore.Store, error) {
	dir, err := storeDir()
	if err != nil {
		return nil, err
	}
	return tokenstore.New(backend, dir)
}

// OpenStore returns the store of a backend, or the first usable one when
// backend is empty.
func OpenStore(backend tokenstore.Backend) (tokenstore.Store, error) {
	if backend != "" {
		return openStore(backend)
	}
	dir, err := storeDir()
	if err != nil {
		return nil, err
	}
	return tokenstore.Auto(dir), nil
}

func saveOpenID(ref OpenID, token Entry) error {
	s, err := openStore(ref.Store)
	if err != nil {
		return err
	}
	b, err := json.Marshal(openIDTokens{
		Issuer:             ref.Issuer,
		ClientID:           ref.ClientID,
		TokenEndpoint:      ref.TokenEndpoint,
		AccessToken:        token.AccessToken,
		AccessTokenExpire:  token.AccessTokenExpire,
		RefreshToken:       token.RefreshToken,
		RefreshTokenExpire: token.RefreshTokenExpire,
	})
	if err != nil {
		return err
	}
	return s.Save(ref.Name(), b)
}

func loadOpenID(ref OpenID) (*Entry, error) {
	s, err := openStore(ref.Store)
	if err != nil {
		return nil, fmt.Errorf("token store %s: %w", ref.Store, err)
	}
	b, err := s.Load(ref.Name())
	if err != nil {
		return nil, fmt.Errorf("token store %s: %w", ref.Store, err)
	}
	var tok openIDTokens
	if err := json.Unmarshal(b, &tok); err != nil {
		return nil, fmt.Errorf("token store %s: %w", ref.Store, err)
	}
	if tok.Issuer != ref.Issuer || tok.ClientID != ref.ClientID {
		return nil, fmt.Errorf("token store %s: the entry %s is the one of issuer %s client %s", ref.Store, ref.Name(), tok.Issuer, tok.ClientID)
	}
	if ref.TokenEndpoint == "" {
		ref.TokenEndpoint = tok.TokenEndpoint
	}
	return &Entry{
		AccessToken:        tok.AccessToken,
		AccessTokenExpire:  tok.AccessTokenExpire,
		RefreshToken:       tok.RefreshToken,
		RefreshTokenExpire: tok.RefreshTokenExpire,
		OpenID:             &ref,
	}, nil
}

func deleteOpenID(ref OpenID) error {
	s, err := openStore(ref.Store)
	if err != nil {
		return err
	}
	return s.Delete(ref.Name())
}

// FindOpenID returns the tokens some context logged in at the issuer and
// client, in any usable store, so another context of the same issuer and
// client logs in with them, without a challenge. It returns nil when there
// are none.
func FindOpenID(issuer, clientID string) *Entry {
	for _, backend := range tokenstore.Backends {
		ref := OpenID{Issuer: issuer, ClientID: clientID, Store: backend}
		if tok, err := loadOpenID(ref); err == nil {
			return tok
		}
	}
	return nil
}

// openIDRef returns the openid reference in the file of a context, nil when
// it holds none.
func openIDRef(contextName string) *OpenID {
	filename, _ := homedir.Expand(FmtFilename(contextName))
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil
	}
	var e Entry
	if err := json.Unmarshal(b, &e); err != nil {
		return nil
	}
	return e.OpenID
}

// openIDReferenced says whether a context still refers to the tokens of an
// issuer and client.
func openIDReferenced(ref OpenID) bool {
	files, err := getAllFiles()
	if err != nil {
		return true
	}
	for _, file := range files {
		name := file.Name()
		if file.IsDir() || !strings.HasPrefix(name, "token-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		other := openIDRef(strings.TrimSuffix(strings.TrimPrefix(name, "token-"), ".json"))
		if other != nil && other.Name() == ref.Name() && other.Store == ref.Store {
			return true
		}
	}
	return false
}

// SaveShared saves the tokens of an openid entry in the store its reference
// names, for the contexts reading them there, without touching the file of
// any context.
func SaveShared(token Entry) error {
	if token.OpenID == nil {
		return fmt.Errorf("not an openid token")
	}
	return saveOpenID(*token.OpenID, token)
}
