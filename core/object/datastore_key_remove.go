package object

import (
	"github.com/opensvc/om3/util/key"
)

// RemoveKey removes a keyword from object
func (t *dataStore) TransactionRemoveKey(keyname string) error {
	k := key.New(dataSectionName, keyname)
	return t.config.PrepareUnset(k)
}

// RemoveKey removes a keyword from object and commits immediately
func (t *dataStore) RemoveKey(keyname string) error {
	k := key.New(dataSectionName, keyname)
	return t.config.Unset(k)
}
