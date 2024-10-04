package object

import (
	"fmt"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/util/key"
)

// TransactionRenameKey changes the key name and return uncommited
func (t *keystore) TransactionRenameKey(name, to string) error {
	if t.HasKey(to) {
		return fmt.Errorf("%w: %s", KeystoreErrExist, to)
	}
	k1 := key.New(dataSectionName, name)
	k2 := key.New(dataSectionName, to)
	v, err := t.config.GetStrict(k1)
	if err != nil {
		return err
	}
	err = t.config.PrepareSet(keyop.T{
		Key:   k2,
		Op:    keyop.Set,
		Value: v,
	})
	if err != nil {
		return err
	}
	return t.config.PrepareUnset(k1)
}

// RenameKey changes the key name and commits immediately
func (t *keystore) RenameKey(name, to string) error {
	if err := t.TransactionRenameKey(name, to); err != nil {
		return err
	}
	return t.config.Commit()
}
