package object

import "opensvc.com/opensvc/util/key"

const (
	// DataSectionName is the name of the section hosting keys in the sec, cfg and usr objects' configuration file.
	DataSectionName = "data"
)

type (
	// Keystore is the base type of sec, cfg and usr objects
	Keystore struct {
		Base
	}

	CustomDecoder interface {
		CustomDecode(string) ([]byte, error)
	}
	CustomEncoder interface {
		CustomEncode([]byte) (string, error)
	}
)

func keyFromName(name string) key.T {
	return key.New(DataSectionName, name)
}

func (t Keystore) HasKey(name string) bool {
	k := keyFromName(name)
	return t.config.HasKey(k)
}
