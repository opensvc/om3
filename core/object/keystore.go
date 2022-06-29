package object

import (
	"io/ioutil"
	"os"

	"opensvc.com/opensvc/util/key"
)

const (
	// dataSectionName is the name of the section hosting keys in the sec, cfg and usr objects' configuration file.
	dataSectionName = "data"
)

type (
	EncodeFunc func([]byte) (string, error)
	DecodeFunc func(string) ([]byte, error)

	// Keystore is the base type of sec, cfg and usr objects
	Keystore struct {
		Base
		CustomEncode EncodeFunc
		CustomDecode DecodeFunc
	}
)

func (t Keystore) Add(options OptsAdd) error {
	return t.add(options.Key, options.From, options.Value)
}

func (t Keystore) Change(options OptsAdd) error {
	return t.change(options.Key, options.From, options.Value)
}

func (t Keystore) Decode(options OptsDecode) ([]byte, error) {
	return t.decode(options.Key)
}

func keyFromName(name string) key.T {
	return key.New(dataSectionName, name)
}

func (t Keystore) HasKey(name string) bool {
	k := keyFromName(name)
	return t.config.HasKey(k)
}

func (t Keystore) temporaryKeyFile(name string) (f *os.File, err error) {
	var (
		b []byte
	)
	if b, err = t.decode(name); err != nil {
		return
	}
	if f, err = ioutil.TempFile(t.Base.paths.tmpDir, ".TemporaryKeyFile.*"); err != nil {
		return
	}
	if _, err = f.Write(b); err != nil {
		return
	}
	return
}

func (t Keystore) postCommit() error {
	return t.postInstall("")
}
