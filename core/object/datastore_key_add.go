package object

import (
	"errors"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/util/key"
)

var (
	ErrKeyExist    = errors.New("key already exists")
	ErrKeyEmpty    = errors.New("key name is empty")
	ErrKeyNotExist = errors.New("key does not exist")
	ErrValueTooBig = errors.New("key value exceeds the allowed size")
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

// TransactionChangeOrAddKey changes the value of an existing key or adds the value to a new key
func (t *dataStore) TransactionChangeOrAddKey(name string, b []byte) error {
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
	keysize, err := t.node.mergedConfig.GetSizeStrict(key.New("node", "max_key_size"))
	if err != nil {
		return err
	}
	if len(b) > int(*keysize) {
		return ErrValueTooBig
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
