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
	encodeFunc func([]byte) (string, error)
	decodeFunc func(string) ([]byte, error)

	// keystore is the base type of sec, cfg and usr objects
	keystore struct {
		core
		customEncode encodeFunc
		customDecode decodeFunc
	}
)

func (t keystore) Add(options OptsAdd) error {
	return t.add(options.Key, options.From, options.Value)
}

func (t keystore) Change(options OptsAdd) error {
	return t.change(options.Key, options.From, options.Value)
}

func (t keystore) Decode(options OptsDecode) ([]byte, error) {
	return t.decode(options.Key)
}

func keyFromName(name string) key.T {
	return key.New(dataSectionName, name)
}

func (t keystore) HasKey(name string) bool {
	k := keyFromName(name)
	return t.config.HasKey(k)
}

func (t keystore) temporaryKeyFile(name string) (f *os.File, err error) {
	var (
		b []byte
	)
	if b, err = t.decode(name); err != nil {
		return
	}
	if f, err = ioutil.TempFile(t.core.paths.tmpDir, ".TemporaryKeyFile.*"); err != nil {
		return
	}
	if _, err = f.Write(b); err != nil {
		return
	}
	return
}

func (t keystore) postCommit() error {
	return t.postInstall("")
}
