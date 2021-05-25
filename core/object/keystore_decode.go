package object

import (
	"fmt"
	"os"
)

const (
	DefaultInstalledFileMode os.FileMode = 0644
)

// OptsDecode is the options of the Decode function of all keystore objects.
type OptsDecode struct {
	Global OptsGlobal
	Lock   OptsLocking
	Key    string `flag:"key"`
}

// Get returns a keyword value
func (t *Keystore) decode(keyname string) ([]byte, error) {
	var (
		s   string
		err error
	)
	if keyname == "" {
		return []byte{}, fmt.Errorf("key name can not be empty")
	}
	if !t.HasKey(keyname) {
		return []byte{}, fmt.Errorf("key does not exist: %s", keyname)
	}
	k := keyFromName(keyname)
	if s, err = t.config.GetStringStrict(k); err != nil {
		return []byte{}, err
	}
	return t.CustomDecode(s)
}
