// Package tokenstore keeps the secrets a client caches between its runs, as
// the refresh token of a login, in the most protected place the machine
// offers.
//
// Three backends are known, tried in this order when none is asked:
//
//   - keyring: the keyring of the user session, the Secret Service of a
//     linux desktop, the macOS keychain or the Windows credential manager.
//     A headless server usually has none.
//
//   - agent: a file encrypted with a key derived from a signature the
//     ssh-agent of the user makes, with an ed25519 or an rsa key, whose
//     signatures are the same each time. The file is of no use without the
//     agent holding that key: a backup, a copied home directory or a stolen
//     disk do not open it. Whoever can use the agent can, as they can use the
//     key itself.
//
//   - file: a file only the user reads. It is what a token is worth on a
//     machine whose other users the user trusts not to be root.
package tokenstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

type (
	// Backend names a place secrets are kept in.
	Backend string

	// Store keeps named secrets in a backend.
	Store interface {
		Backend() Backend
		Load(name string) ([]byte, error)
		Save(name string, b []byte) error
		Delete(name string) error
	}
)

const (
	Keyring Backend = "keyring"
	Agent   Backend = "agent"
	File    Backend = "file"
)

var (
	// Backends are the backends, in the order Auto tries them.
	Backends = []Backend{Keyring, Agent, File}

	// ErrNotFound is returned on the load of a secret the store does not
	// hold.
	ErrNotFound = errors.New("not found in the token store")

	nameRegexp = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// New returns the store of a backend, keeping its files under dir, or an
// error saying why the backend can not be used on this machine.
func New(backend Backend, dir string) (Store, error) {
	switch backend {
	case Keyring:
		return newKeyringStore()
	case Agent:
		return newAgentStore(dir)
	case File:
		return &fileStore{dir: dir}, nil
	default:
		return nil, fmt.Errorf("unknown token store %q: expected one of %v", backend, Backends)
	}
}

// Auto returns the store of the first backend usable on this machine, the
// file one being always usable.
func Auto(dir string) Store {
	for _, backend := range Backends {
		if s, err := New(backend, dir); err == nil {
			return s
		}
	}
	return &fileStore{dir: dir}
}

// ParseBackend returns the backend s names, Auto being the empty string.
func ParseBackend(s string) (Backend, error) {
	switch b := Backend(s); b {
	case "", Keyring, Agent, File:
		return b, nil
	default:
		return "", fmt.Errorf("unknown token store %q: expected one of %v", s, Backends)
	}
}

func checkName(name string) error {
	if !nameRegexp.MatchString(name) {
		return fmt.Errorf("invalid token store entry name %q", name)
	}
	return nil
}

// fileStore keeps the secrets in files only the user reads.
type fileStore struct {
	dir string
}

func (t *fileStore) Backend() Backend { return File }

func (t *fileStore) path(name string) string {
	return filepath.Join(t.dir, name+".json")
}

func (t *fileStore) Load(name string) ([]byte, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(t.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return b, err
}

func (t *fileStore) Save(name string, b []byte) error {
	if err := checkName(name); err != nil {
		return err
	}
	return writePrivate(t.path(name), b)
}

func (t *fileStore) Delete(name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	return removeIfExists(t.path(name))
}

// writePrivate writes a file only its owner reads, replacing the one there
// at once: a reader never sees half a file.
func writePrivate(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
