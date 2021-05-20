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

func (t *Keystore) add(name string, from string, value string, ce CustomEncoder) error {
	if name == "" {
		return fmt.Errorf("key name can not be empty")
	}
	if t.HasKey(name) {
		return fmt.Errorf("key already exist: %s. use the change action.", name)
	}
	return t.alter(name, from, value, ce)
}

func (t *Keystore) change(name string, from string, value string, ce CustomEncoder) error {
	if name == "" {
		return fmt.Errorf("key name can not be empty")
	}
	return t.alter(name, from, value, ce)
}

func (t *Keystore) alter(name string, from string, value string, ce CustomEncoder) error {
	var (
		err error
	)
	switch {
	case from != "":
		u := uri.New(from)
		switch {
		case u.IsValid():
			err = t.fromURI(name, u, ce)
		case file.ExistsAndRegular(from):
			err = t.fromRegular(name, from, ce)
		case file.ExistsAndDir(from):
			err = t.fromDir(name, from, ce)
		default:
			err = fmt.Errorf("unexpected value source: %s", from)
		}
	default:
		err = t.fromValue(name, value, ce)
	}
	if err != nil {
		return err
	}
	return t.config.Commit()
}

func (t *Keystore) fromValue(name string, value string, ce CustomEncoder) error {
	b := []byte(value)
	return t.addKey(name, b, ce)
}

func (t *Keystore) fromRegular(name string, p string, ce CustomEncoder) error {
	b, err := file.ReadAll(p)
	if err != nil {
		return err
	}
	return t.addKey(name, b, ce)
}

func (t *Keystore) fromDir(name string, p string, ce CustomEncoder) error {
	// TODO: walk and call fromRegular
	return nil
}

func (t *Keystore) fromURI(name string, u uri.T, ce CustomEncoder) error {
	fName, err := u.Fetch()
	if err != nil {
		return err
	}
	defer os.Remove(fName)
	return t.fromRegular(name, fName, ce)
}

// Note: addKey does not commit, so it can be used multiple times efficiently.
func (t *Keystore) addKey(name string, b []byte, ce CustomEncoder) error {
	s, err := ce.CustomEncode(b)
	if err != nil {
		return err
	}
	op := keyop.T{
		Key:   keyFromName(name),
		Op:    keyop.Set,
		Value: s,
	}
	return t.config.Set(op)
}
