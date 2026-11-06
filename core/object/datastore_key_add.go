package object

import (
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/sizeconv"
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

// AddKeySeeded sets a new key the install of the resource rid of the object
// p seeded, and commits immediately. The consumers of the datastore are
// refreshed but that resource, whose install installs the key next: a
// refresh of it, nested in its install, would run its install again.
//
// An empty rid is the object seeding the key before its resources act, at
// its provision or its start: none of its resources is refreshed, as they
// install the key next.
func (t *dataStore) AddKeySeeded(name string, b []byte, p naming.Path, rid string) error {
	if err := t.TransactionAddKey(name, b); err != nil {
		return err
	}
	t.skipRefresh = func(path naming.Path, r string) bool { return path == p && (rid == "" || r == rid) }
	defer func() { t.skipRefresh = nil }()
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
	keysize, err := t.maxKeySize()
	if err != nil {
		return err
	}
	if int64(len(b)) > keysize {
		return ValueTooBig(int64(len(b)), keysize)
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

// maxKeySize returns the most a key value holds, node.max_key_size.
func (t *dataStore) maxKeySize() (int64, error) {
	v, err := t.node.mergedConfig.GetSizeStrict(key.New("node", "max_key_size"))
	if err != nil {
		return 0, err
	}
	return *v, nil
}

// ValueTooBig is the error of a key value of size bytes, over the max allowed
// by node.max_key_size: it says both sizes and the keyword to raise, the
// sentinel ErrValueTooBig wrapped for errors.Is.
func ValueTooBig(size, max int64) error {
	return fmt.Errorf("%w: %sB > %sB (raise node.max_key_size)", ErrValueTooBig, sizeconv.BSize(float64(size)), sizeconv.BSize(float64(max)))
}
