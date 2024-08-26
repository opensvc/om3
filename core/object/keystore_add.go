package object

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/uri"
)

var (
	KeystoreErrExist              = errors.New("key already exists")
	KeystoreErrKeyEmpty           = errors.New("key is empty")
	KeystoreErrNotExist           = errors.New("key does not exist")
	KeystoreErrValueSourceUnknown = errors.New("unknown value source")
	KeystoreErrValueSourceEmpty   = errors.New("value source is empty")
)

// TransactionAddKey sets a new key
func (t *keystore) TransactionAddKey(name string, b []byte) error {
	if t.HasKey(name) {
		return KeystoreErrExist
	}
	return t.addKey(name, b)
}

// AddKey sets a new key and commits immediately
func (t *keystore) AddKey(name string, b []byte) error {
	if err := t.TransactionAddKey(name, b); err != nil {
		return err
	}
	return t.config.Commit()
}

// TransactionChangeKey changes the value of a existing key
func (t *keystore) TransactionChangeKey(name string, b []byte) error {
	if !t.HasKey(name) {
		return KeystoreErrNotExist
	}
	return t.addKey(name, b)
}

// ChangeKey changes the value of a existing key and commits immediately
func (t *keystore) ChangeKey(name string, b []byte) error {
	if err := t.TransactionChangeKey(name, b); err != nil {
		return err
	}
	return t.config.Commit()
}
func (t *keystore) AddKeyFrom(name string, from string) error {
	if name == "" {
		return KeystoreErrKeyEmpty
	}
	if t.HasKey(name) {
		return KeystoreErrExist
	}
	if err := t.alterFrom(name, from); err != nil {
		return err
	}
	return t.config.Commit()
}

func (t *keystore) ChangeKeyFrom(name string, from string) error {
	if name == "" {
		return KeystoreErrKeyEmpty
	}
	if !t.HasKey(name) {
		return KeystoreErrNotExist
	}
	if err := t.alterFrom(name, from); err != nil {
		return err
	}
	return t.config.Commit()
}

func (t *keystore) alterFrom(name string, from string) error {
	switch from {
	case "":
		return KeystoreErrValueSourceEmpty
	case "-", "stdin", "/dev/stdin":
		return t.fromStdin(name)
	default:
		u := uri.New(from)
		if u.IsValid() {
			return t.fromURI(name, u)
		}
		if v, err := file.ExistsAndRegular(from); err != nil {
			return err
		} else if v {
			return t.fromRegular(name, from)
		}
		if v, err := file.ExistsAndDir(from); err != nil {
			return err
		} else if v {
			return t.fromDir(name, from)
		}
		return KeystoreErrValueSourceUnknown
	}
}

func (t *keystore) fromStdin(name string) error {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		reader := bufio.NewReader(os.Stdin)
		b, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		return t.addKey(name, b)
	} else {
		return fmt.Errorf("stdin must be a pipe")
	}
}

func (t *keystore) fromRegular(name string, p string) error {
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	return t.addKey(name, b)
}

func (t *keystore) fromDir(name string, p string) error {
	// TODO: walk and call fromRegular
	return nil
}

func (t *keystore) fromURI(name string, u uri.T) error {
	fName, err := u.Fetch()
	if err != nil {
		return err
	}
	defer os.Remove(fName)
	return t.fromRegular(name, fName)
}

// Note: addKey does not commit, so it can be used multiple times efficiently.
func (t *keystore) addKey(name string, b []byte) error {
	if name == "" {
		return KeystoreErrKeyEmpty
	}
	if b == nil {
		b = []byte{}
	}
	s, err := t.customEncode(b)
	if err != nil {
		return err
	}
	op := keyop.T{
		Key:   keyFromName(name),
		Op:    keyop.Set,
		Value: s,
	}
	if err := t.config.PrepareSet(op); err != nil {
		return err
	}
	if t.config.Changed() {
		t.log.Attr("key", name).Attr("len", len(s)).Infof("set key %s", name)
	}
	return nil
}
