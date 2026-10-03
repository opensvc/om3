package tokencache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/tokenstore"
)

func withHome(t *testing.T) string {
	home := t.TempDir()
	t.Setenv("HOME", home)
	homedir.Reset()
	t.Cleanup(homedir.Reset)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "opensvc"), 0o700))
	return home
}

// The contexts logged in at the same issuer and client share their tokens:
// a refresh saved by one is the token of the other, and the file of a context
// holds no token.
func TestOpenIDTokensAreShared(t *testing.T) {
	home := withHome(t)
	ref := &OpenID{Issuer: "https://idp/app/", ClientID: "om3", TokenEndpoint: "https://idp/token", Store: tokenstore.File}
	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	require.NoError(t, Save("c1", Entry{AccessToken: "a1", AccessTokenExpire: exp, RefreshToken: "r1", OpenID: ref}))
	require.NoError(t, Save("c2", Entry{AccessToken: "a1", AccessTokenExpire: exp, RefreshToken: "r1", OpenID: ref}))

	raw, err := os.ReadFile(filepath.Join(home, ".config", "opensvc", "token-c1.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "r1", "the context file holds a reference, no token")

	require.NoError(t, Save("c1", Entry{AccessToken: "a2", AccessTokenExpire: exp, RefreshToken: "r2", OpenID: ref}))
	tok, err := Load("c2")
	require.NoError(t, err)
	assert.Equal(t, "a2", tok.AccessToken, "the refresh of c1 is the token of c2")
	assert.Equal(t, "r2", tok.RefreshToken)
	assert.Equal(t, "https://idp/token", tok.OpenID.TokenEndpoint)
	assert.True(t, tok.HasValidRefresh(time.Now()), "a refresh token of unknown expiry is tried")

	found := FindOpenID("https://idp/app/", "om3")
	require.NotNil(t, found)
	assert.Equal(t, "a2", found.AccessToken)
	assert.Nil(t, FindOpenID("https://idp/app/", "other"))

	require.NoError(t, Delete("c1"))
	_, err = Load("c2")
	assert.NoError(t, err, "the tokens are kept for the context still using them")
	require.NoError(t, Delete("c2"))
	assert.Nil(t, FindOpenID("https://idp/app/", "om3"), "and forgotten with the last")
}

// A context logged in with a password keeps its tokens in its own file.
func TestPasswordTokensStayInTheContextFile(t *testing.T) {
	withHome(t)
	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	require.NoError(t, Save("c1", Entry{AccessToken: "a1", AccessTokenExpire: exp, RefreshToken: "r1", RefreshTokenExpire: exp}))
	tok, err := Load("c1")
	require.NoError(t, err)
	assert.Equal(t, "r1", tok.RefreshToken)
	assert.Nil(t, tok.OpenID)
	assert.False(t, Entry{RefreshToken: "r", RefreshTokenExpire: time.Now().Add(-time.Minute)}.HasValidRefresh(time.Now()))
}
