package object

import (
	"fmt"
)

// Get returns a keyword value
func (t *keystore) decode(keyname string) ([]byte, error) {
	var (
		s   string
		err error
	)
	if keyname == "" {
		return []byte{}, KeystoreErrKeyEmpty
	}
	if !t.HasKey(keyname) {
		return []byte{}, fmt.Errorf("%w: %s", KeystoreErrNotExist, keyname)
	}
	k := keyFromName(keyname)
	if s, err = t.config.GetStrict(k); err != nil {
		return []byte{}, err
	}
	return t.customDecode(s)
}

// DecodeKey returns the decoded bytes of the key value
func (t *keystore) DecodeKey(keyname string) ([]byte, error) {
	return t.decode(keyname)
}
