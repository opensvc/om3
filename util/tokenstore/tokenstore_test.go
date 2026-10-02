package tokenstore

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/agent"
)

func testAgent(t *testing.T, keys ...any) agent.ExtendedAgent {
	t.Helper()
	a := agent.NewKeyring().(agent.ExtendedAgent)
	for _, k := range keys {
		require.NoError(t, a.Add(agent.AddedKey{PrivateKey: k}))
	}
	return a
}

func ed25519Key(t *testing.T) ed25519.PrivateKey {
	_, k, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return k
}

// A secret saved in a store is the one loaded back, and none is found once
// deleted.
func TestStoresRoundTrip(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for name, s := range map[string]Store{
		"file":          &fileStore{dir: t.TempDir()},
		"agent ed25519": newAgentStoreWith(t.TempDir(), testAgent(t, ed25519Key(t))),
		"agent rsa":     newAgentStoreWith(t.TempDir(), testAgent(t, rsaKey)),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := s.Load("openid-abc")
			assert.ErrorIs(t, err, ErrNotFound)
			require.NoError(t, s.Save("openid-abc", []byte("refresh1")))
			b, err := s.Load("openid-abc")
			require.NoError(t, err)
			assert.Equal(t, "refresh1", string(b))
			require.NoError(t, s.Save("openid-abc", []byte("refresh2")))
			b, err = s.Load("openid-abc")
			require.NoError(t, err)
			assert.Equal(t, "refresh2", string(b))
			require.NoError(t, s.Delete("openid-abc"))
			_, err = s.Load("openid-abc")
			assert.ErrorIs(t, err, ErrNotFound)
			assert.NoError(t, s.Delete("openid-abc"), "deleting what is not there is no error")
		})
	}
}

// The file of the file store is readable by its owner alone.
func TestFileStoreIsPrivate(t *testing.T) {
	s := &fileStore{dir: t.TempDir()}
	require.NoError(t, s.Save("x", []byte("secret")))
	st, err := os.Stat(s.path("x"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
}

// The file of the agent store holds no clear secret, and is of no use
// without the agent key it was encrypted with, or once altered.
func TestAgentStoreProtects(t *testing.T) {
	dir := t.TempDir()
	key := ed25519Key(t)
	s := newAgentStoreWith(dir, testAgent(t, key))
	require.NoError(t, s.Save("openid-abc", []byte("the-refresh-token")))
	raw, err := os.ReadFile(filepath.Join(dir, "openid-abc.agent"))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "the-refresh-token")

	other := newAgentStoreWith(dir, testAgent(t, ed25519Key(t)))
	_, err = other.Load("openid-abc")
	assert.ErrorIs(t, err, ErrAgentKeyMissing, "another agent can not read it")

	again := newAgentStoreWith(dir, testAgent(t, key))
	b, err := again.Load("openid-abc")
	require.NoError(t, err)
	assert.Equal(t, "the-refresh-token", string(b), "the same key in another agent reads it, as a forwarded one")

	var f agentFile
	require.NoError(t, json.Unmarshal(raw, &f))
	f.Ciphertext[0] ^= 1
	altered, _ := json.Marshal(f)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "openid-abc.agent"), altered, 0o600))
	_, err = again.Load("openid-abc")
	assert.ErrorContains(t, err, "decrypt")

	require.NoError(t, again.Save("openid-abc", []byte("x")))
	require.NoError(t, os.Rename(filepath.Join(dir, "openid-abc.agent"), filepath.Join(dir, "openid-def.agent")))
	_, err = again.Load("openid-def")
	assert.Error(t, err, "an entry renamed is not the one it was encrypted as")
}

// An agent holding only a key whose signatures vary at each call can not
// derive a key, and is no store.
func TestAgentStoreNeedsADeterministicKey(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	s := newAgentStoreWith(t.TempDir(), testAgent(t, ecKey))
	_, err = s.signingKey()
	assert.ErrorContains(t, err, "no ed25519 or rsa key")
	assert.Error(t, s.Save("x", []byte("y")))
}

// A name is a plain word: no path leads out of the store directory.
func TestNamesAreChecked(t *testing.T) {
	s := &fileStore{dir: t.TempDir()}
	for _, name := range []string{"", "../x", "a/b", "a b"} {
		assert.Error(t, s.Save(name, []byte("y")), name)
	}
	_, err := ParseBackend("vault")
	assert.Error(t, err)
	b, err := ParseBackend("")
	require.NoError(t, err)
	assert.Equal(t, Backend(""), b)
}
