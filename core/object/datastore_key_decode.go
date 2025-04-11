package object

import (
	"fmt"
)

// decode returns a keyword value
func (t *dataStore) decode(keyname string) ([]byte, error) {
	var (
		s   string
		err error
	)
	if keyname == "" {
		return []byte{}, ErrKeyEmpty
	}
	if !t.HasKey(keyname) {
		return []byte{}, fmt.Errorf("%w: %s", ErrKeyNotExist, keyname)
	}
	k := keyFromName(keyname)
	if s, err = t.config.GetStrict(k); err != nil {
		return []byte{}, err
	}
	return t.encodeDecoder.Decode(s)
}

// DecodeKey returns the decoded bytes of the key value
func (t *dataStore) DecodeKey(keyname string) ([]byte, error) {
	return t.decode(keyname)
}
