package object

import (
	"github.com/opensvc/om3/util/key"
)

// Remove gets a keyword value
func (t *keystore) RemoveKey(keyname string) error {
	k := key.New(dataSectionName, keyname)
	return unsetKeys(t.config, k)
}
