package tokenstore

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// keyringService is the service the secrets are kept under in the keyring.
const keyringService = "opensvc-token-cache"

type keyringStore struct{}

// newKeyringStore returns the keyring store when the session has a keyring:
// a lookup of a secret nobody saved answers "not found" there, and an error
// where there is no keyring to look in.
func newKeyringStore() (Store, error) {
	_, err := keyring.Get(keyringService, "probe")
	switch {
	case err == nil, errors.Is(err, keyring.ErrNotFound):
		return keyringStore{}, nil
	default:
		return nil, fmt.Errorf("no keyring in this session: %w", err)
	}
}

func (keyringStore) Backend() Backend { return Keyring }

func (keyringStore) Load(name string) ([]byte, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}
	s, err := keyring.Get(keyringService, name)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, ErrNotFound
	}
	return []byte(s), err
}

func (keyringStore) Save(name string, b []byte) error {
	if err := checkName(name); err != nil {
		return err
	}
	return keyring.Set(keyringService, name, string(b))
}

func (keyringStore) Delete(name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	err := keyring.Delete(keyringService, name)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
