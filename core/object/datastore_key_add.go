package object

import (
	"errors"

	"github.com/opensvc/om3/core/keyop"
)

var (
	ErrKeyExist    = errors.New("key already exists")
	ErrKeyEmpty    = errors.New("key name is empty")
	ErrKeyNotExist = errors.New("key does not exist")
)

// TransactionAddKey sets a new key
func (t *dataStore) TransactionAddKey(name string, b []byte) error {
	if t.HasKey(name) {
		return ErrKeyExist
	}
	return t.addKey(name, b)
}

// AddKey sets a new key and commits immediately
func (t *dataStore) AddKey(name string, b []byte) error {
	if err := t.TransactionAddKey(name, b); err != nil {
		return err
	}
	return t.config.Commit()
}

// TransactionChangeKey inserts or updates the value of a existing key
func (t *dataStore) TransactionChangeKey(name string, b []byte) error {
	return t.addKey(name, b)
}

// ChangeKey changes the value of a existing key and commits immediately
func (t *dataStore) ChangeKey(name string, b []byte) error {
	if err := t.TransactionChangeKey(name, b); err != nil {
		return err
	}
	return t.config.Commit()
}

// Note: addKey does not commit, so it can be used multiple times efficiently.
func (t *dataStore) addKey(name string, b []byte) error {
	if name == "" {
		return ErrKeyEmpty
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
