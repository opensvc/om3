package object

import (
	"os"

	"github.com/opensvc/om3/util/key"
)

const (
	// dataSectionName is the name of the section hosting keys in the sec, cfg and usr objects' configuration file.
	dataSectionName = "data"
)

type (
	encodeFunc func([]byte) (string, error)
	decodeFunc func(string) ([]byte, error)

	keystore struct {
		core
		customEncode encodeFunc
		customDecode decodeFunc
	}

	// Keystore is the base interface of sec, cfg and usr objects
	Keystore interface {
		Core
		HasKey(name string) bool
		AddKey(name string, b []byte) error
		AddKeyFrom(name string, from string) error
		ChangeKey(name string, b []byte) error
		ChangeKeyFrom(name string, from string) error
		DecodeKey(keyname string) ([]byte, error)
		AllKeys() ([]string, error)
		MatchingKeys(string) ([]string, error)
		RemoveKey(name string) error
		EditKey(name string) error
		InstallKey(name string) error
		InstallKeyTo(string, string, *os.FileMode, *os.FileMode, string, string) error

		TransactionAddKey(name string, b []byte) error
		TransactionChangeKey(name string, b []byte) error
		TransactionRemoveKey(name string) error
	}

	// SecureKeystore is implemented by encrypting Keystore object kinds (usr, sec).
	SecureKeystore interface {
		GenCert() error
		PKCS() ([]byte, error)
	}
)

func keyFromName(name string) key.T {
	return key.New(dataSectionName, name)
}

func (t *keystore) HasKey(name string) bool {
	k := keyFromName(name)
	return t.config.HasKey(k)
}

func (t *keystore) temporaryKeyFile(name string) (f *os.File, err error) {
	var (
		b []byte
	)
	if b, err = t.decode(name); err != nil {
		return
	}
	if f, err = os.CreateTemp(t.core.paths.tmpDir, ".TemporaryKeyFile.*"); err != nil {
		return
	}
	if _, err = f.Write(b); err != nil {
		return
	}
	return
}

func (t *keystore) postCommit() error {
	return t.postInstall("")
}
