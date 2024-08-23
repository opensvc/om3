package object

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/uri"
)

// TransactionAddKey sets a new key
func (t *keystore) TransactionAddKey(name string, b []byte) error {
	if t.HasKey(name) {
		return fmt.Errorf("key already exist: %s. use the change action", name)
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
		return fmt.Errorf("key does not exist: %s. use the add action", name)
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
		return fmt.Errorf("key name can not be empty")
	}
	if t.HasKey(name) {
		return fmt.Errorf("key already exist: %s. use the change action", name)
	}
	if err := t.alterFrom(name, from); err != nil {
		return err
	}
	return t.config.Commit()
}

func (t *keystore) ChangeKeyFrom(name string, from string) error {
	if name == "" {
		return fmt.Errorf("key name can not be empty")
	}
	if !t.HasKey(name) {
		return fmt.Errorf("key does not exist: %s. use the add action", name)
	}
	if err := t.alterFrom(name, from); err != nil {
		return err
	}
	return t.config.Commit()
}

func (t *keystore) alterFrom(name string, from string) error {
	switch from {
	case "":
		return fmt.Errorf("empty value source")
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
		return fmt.Errorf("unexpected value source: %s", from)
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
		return fmt.Errorf("key name can not be empty")
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
