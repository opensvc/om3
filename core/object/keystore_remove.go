package object

import (
	"github.com/opensvc/om3/util/key"
)

// RemoveKey removes a keyword from object
func (t *keystore) RemoveKey(keyname string) error {
	k := key.New(dataSectionName, keyname)
	return t.config.Unset(k)
}
