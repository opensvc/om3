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
	OptsLock
	Key   string  `flag:"key"`
	From  *string `flag:"from"`
	Value *string `flag:"value"`
}

func (t *keystore) add(name string, from, value *string) error {
	if name == "" {
		return fmt.Errorf("key name can not be empty")
	}
	if t.HasKey(name) {
		if value == nil && from == nil {
			return nil
		}
		return fmt.Errorf("key already exist: %s. use the change action.", name)
	}
	return t.alter(name, from, value)
}

func (t *keystore) change(name string, from, value *string) error {
	if name == "" {
		return fmt.Errorf("key name can not be empty")
	}
	return t.alter(name, from, value)
}

func (t *keystore) alter(name string, from, value *string) error {
	var (
		err error
	)
	switch {
	case from != nil && *from != "":
		u := uri.New(*from)
		switch {
		case u.IsValid():
			err = t.fromURI(name, u)
		case file.ExistsAndRegular(*from):
			err = t.fromRegular(name, *from)
		case file.ExistsAndDir(*from):
			err = t.fromDir(name, *from)
		default:
			err = fmt.Errorf("unexpected value source: %s", *from)
		}
	default:
		err = t.fromValue(name, value)
	}
	if err != nil {
		return err
	}
	return t.config.Commit()
}

func (t *keystore) fromValue(name string, value *string) error {
	var b []byte
	if value == nil {
		b = []byte{}
	} else {
		b = []byte(*value)
	}
	return t.addKey(name, b)
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
	s, err := t.customEncode(b)
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
	if t.config.Changed() {
		t.log.Info().Str("key", name).Int("len", len(s)).Msg("key set")
	}
	return nil
}

// AddKey sets a key and commits immediately
func (t *keystore) AddKey(name string, b []byte) error {
	if err := t.addKey(name, b); err != nil {
		return err
	}
	return t.config.Commit()
}
