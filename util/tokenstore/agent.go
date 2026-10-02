package tokenstore

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type (
	// agentStore keeps the secrets in files encrypted with a key derived from
	// a signature of the ssh-agent.
	agentStore struct {
		dir   string
		agent agent.ExtendedAgent
	}

	// agentFile is an encrypted secret, and the agent key it is encrypted
	// with, by fingerprint.
	agentFile struct {
		Fingerprint string `json:"fingerprint"`
		Nonce       []byte `json:"nonce"`
		Ciphertext  []byte `json:"ciphertext"`
	}
)

// ErrAgentKeyMissing is returned on the load of a secret encrypted with a key
// the agent does not hold now.
var ErrAgentKeyMissing = errors.New("the ssh-agent does not hold the key the token was encrypted with")

const agentKeyInfo = "opensvc token store aes-256-gcm"

func newAgentStore(dir string) (Store, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("no ssh-agent: SSH_AUTH_SOCK is not set")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("no ssh-agent: %w", err)
	}
	t := &agentStore{dir: dir, agent: agent.NewClient(conn)}
	if _, err := t.signingKey(); err != nil {
		return nil, err
	}
	return t, nil
}

func newAgentStoreWith(dir string, a agent.ExtendedAgent) *agentStore {
	return &agentStore{dir: dir, agent: a}
}

func (t *agentStore) Backend() Backend { return Agent }

func (t *agentStore) path(name string) string {
	return filepath.Join(t.dir, name+".agent")
}

// signingKey returns the first key of the agent whose signatures are the same
// each time: an ed25519 key, else an rsa key. An ecdsa signature differs at
// each call, and a security key asks for a touch at each call, so neither
// can derive a key to read a file with on the next run.
func (t *agentStore) signingKey() (*agent.Key, error) {
	keys, err := t.agent.List()
	if err != nil {
		return nil, fmt.Errorf("list the ssh-agent keys: %w", err)
	}
	for _, want := range []string{ssh.KeyAlgoED25519, ssh.KeyAlgoRSA} {
		for _, k := range keys {
			if k.Type() == want {
				return k, nil
			}
		}
	}
	return nil, errors.New("the ssh-agent holds no ed25519 or rsa key to derive a token store key from")
}

func (t *agentStore) keyByFingerprint(fp string) (*agent.Key, error) {
	keys, err := t.agent.List()
	if err != nil {
		return nil, fmt.Errorf("list the ssh-agent keys: %w", err)
	}
	for _, k := range keys {
		if ssh.FingerprintSHA256(k) == fp {
			return k, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrAgentKeyMissing, fp)
}

// cipherFor derives the cipher of a secret from the signature the agent makes
// of its name with key. The signature is made twice, and a key whose two
// signatures differ is refused: the file it would encrypt could never be
// read again.
func (t *agentStore) cipherFor(k *agent.Key, name string) (cipher.AEAD, error) {
	data := []byte("opensvc token store v1\n" + name)
	sign := func() ([]byte, error) {
		var flags agent.SignatureFlags
		if k.Type() == ssh.KeyAlgoRSA {
			flags = agent.SignatureFlagRsaSha256
		}
		sig, err := t.agent.SignWithFlags(k, data, flags)
		if err != nil {
			return nil, fmt.Errorf("ssh-agent sign: %w", err)
		}
		return sig.Blob, nil
	}
	sig1, err := sign()
	if err != nil {
		return nil, err
	}
	sig2, err := sign()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(sig1, sig2) {
		return nil, fmt.Errorf("the signatures of the ssh-agent key %s differ at each call, and can not derive a key", ssh.FingerprintSHA256(k))
	}
	key, err := hkdf.Key(sha256.New, sig1, nil, agentKeyInfo, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (t *agentStore) Load(name string) ([]byte, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(t.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	var f agentFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", t.path(name), err)
	}
	k, err := t.keyByFingerprint(f.Fingerprint)
	if err != nil {
		return nil, err
	}
	aead, err := t.cipherFor(k, name)
	if err != nil {
		return nil, err
	}
	plain, err := aead.Open(nil, f.Nonce, f.Ciphertext, []byte(name+"\n"+f.Fingerprint))
	if err != nil {
		return nil, fmt.Errorf("%s: decrypt: %w", t.path(name), err)
	}
	return plain, nil
}

func (t *agentStore) Save(name string, b []byte) error {
	if err := checkName(name); err != nil {
		return err
	}
	k, err := t.signingKey()
	if err != nil {
		return err
	}
	aead, err := t.cipherFor(k, name)
	if err != nil {
		return err
	}
	f := agentFile{Fingerprint: ssh.FingerprintSHA256(k), Nonce: make([]byte, aead.NonceSize())}
	if _, err := rand.Read(f.Nonce); err != nil {
		return err
	}
	f.Ciphertext = aead.Seal(nil, f.Nonce, b, []byte(name+"\n"+f.Fingerprint))
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return writePrivate(t.path(name), data)
}

func (t *agentStore) Delete(name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	return removeIfExists(t.path(name))
}
