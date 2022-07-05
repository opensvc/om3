package object

import (
	"opensvc.com/opensvc/util/key"
)

// Remove gets a keyword value
func (t *keystore) RemoveKey(keyname string) error {
	k := key.New(dataSectionName, keyname)
	return unsetKeys(t.config, k)
}
