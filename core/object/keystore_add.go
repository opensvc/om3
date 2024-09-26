package object

import (
	"errors"

	"github.com/opensvc/om3/core/keyop"
)

var (
	KeystoreErrExist    = errors.New("key already exists")
	KeystoreErrKeyEmpty = errors.New("key is empty")
	KeystoreErrNotExist = errors.New("key does not exist")
)

// TransactionAddKey sets a new key
func (t *keystore) TransactionAddKey(name string, b []byte) error {
	if t.HasKey(name) {
		return KeystoreErrExist
	}
	return t.addKey(name, b)
}

// AddKey sets a new key and commits immediately
func (t *keystore) AddKey(name string, b []byte) error {
	if err := t.TransactionAddKey(name, b); err != nil {
		return err
	}
	return t.config.Commit()
}

// TransactionChangeKey inserts or updates the value of a existing key
func (t *keystore) TransactionChangeKey(name string, b []byte) error {
	return t.addKey(name, b)
}

// ChangeKey changes the value of a existing key and commits immediately
func (t *keystore) ChangeKey(name string, b []byte) error {
	if err := t.TransactionChangeKey(name, b); err != nil {
		return err
	}
	return t.config.Commit()
}

// Note: addKey does not commit, so it can be used multiple times efficiently.
func (t *keystore) addKey(name string, b []byte) error {
	if name == "" {
		return KeystoreErrKeyEmpty
	}
	if b == nil {
		b = []byte{}
	}
	s, err := t.encodeDecoder.Encode(b)
	if err != nil {
		return err
	}
	op := keyop.T{
		Key:   keyFromName(name),
		Op:    keyop.Set,
		Value: s,
	}
	if err := t.config.PrepareSet(op); err != nil {
		return err
	}
	if t.config.Changed() {
		t.log.Attr("key", name).Attr("len", len(s)).Infof("set key %s", name)
	}
	return nil
}
