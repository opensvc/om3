package object

import (
	"opensvc.com/opensvc/util/key"
)

// OptsUnset is the options of the Unset object method.
type OptsRemove struct {
	OptsGlobal
	OptsLock
	Key string `flag:"key"`
}

// Remove gets a keyword value
func (t *Keystore) Remove(options OptsRemove) error {
	return t.RemoveKey(options.Key)
}

// Remove gets a keyword value
func (t *Keystore) RemoveKey(keyname string) error {
	k := key.New(DataSectionName, keyname)
	return t.UnsetKeys(k)
}
