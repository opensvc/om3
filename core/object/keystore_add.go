package object

import (
	"fmt"
	"os"

	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/uri"
)

// OptsAdd is the options of the Decode function of all keystore objects.
type OptsAdd struct {
	Global OptsGlobal
	Lock   OptsLocking
	Key    string `flag:"key"`
	From   string `flag:"from"`
	Value  string `flag:"value"`
}

func (t *Keystore) add(name string, from string, value string) error {
	if name == "" {
		return fmt.Errorf("key name can not be empty")
	}
	if t.HasKey(name) {
		return fmt.Errorf("key already exist: %s. use the change action.", name)
	}
	return t.alter(name, from, value)
}

func (t *Keystore) change(name string, from string, value string) error {
	if name == "" {
		return fmt.Errorf("key name can not be empty")
	}
	return t.alter(name, from, value)
}

func (t *Keystore) alter(name string, from string, value string) error {
	var (
		err error
	)
	switch {
	case from != "":
		u := uri.New(from)
		switch {
		case u.IsValid():
			err = t.fromURI(name, u)
		case file.ExistsAndRegular(from):
			err = t.fromRegular(name, from)
		case file.ExistsAndDir(from):
			err = t.fromDir(name, from)
		default:
			err = fmt.Errorf("unexpected value source: %s", from)
		}
	default:
		err = t.fromValue(name, value)
	}
	if err != nil {
		return err
	}
	return t.config.Commit()
}

func (t *Keystore) fromValue(name string, value string) error {
	b := []byte(value)
	return t.addKey(name, b)
}

func (t *Keystore) fromRegular(name string, p string) error {
	b, err := file.ReadAll(p)
	if err != nil {
		return err
	}
	return t.addKey(name, b)
}

func (t *Keystore) fromDir(name string, p string) error {
	// TODO: walk and call fromRegular
	return nil
}

func (t *Keystore) fromURI(name string, u uri.T) error {
	fName, err := u.Fetch()
	if err != nil {
		return err
	}
	defer os.Remove(fName)
	return t.fromRegular(name, fName)
}

// Note: addKey does not commit, so it can be used multiple times efficiently.
func (t *Keystore) addKey(name string, b []byte) error {
	s, err := t.CustomEncode(b)
	if err != nil {
		return err
	}
	op := keyop.T{
		Key:   keyFromName(name),
		Op:    keyop.Set,
		Value: s,
	}
	if err := t.config.Set(op); err != nil {
		return err
	}
	t.log.Info().Str("key", name).Int("len", len(s)).Msg("key set")
	return nil
}

// AddKey sets a key and commits immediately
func (t *Keystore) AddKey(name string, b []byte) error {
	if err := t.addKey(name, b); err != nil {
		return err
	}
	return t.config.Commit()
}
