package object

import (
	"opensvc.com/opensvc/util/key"
)

// OptsUnset is the options of the Unset object method.
type OptsRemove struct {
	Global OptsGlobal
	Lock   OptsLocking
	Key    string `flag:"key"`
}

// Remove gets a keyword value
func (t *Keystore) Remove(options OptsRemove) error {
	k := key.New(DataSectionName, options.Key)
	return t.UnsetKeys(k)
}
