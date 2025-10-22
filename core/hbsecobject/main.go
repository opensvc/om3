// Package hbsecobject provides utilities to manage the naming.SecHb object
package hbsecobject

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/opensvc/om3/core/hbsecret"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
)

func Get() (sec *hbsecret.Secret, err error) {
	var (
		b []byte

		store object.DataStore

		key, altKey         string
		version, altVersion uint64
	)
	store, err = object.NewSec(naming.SecHb, object.WithVolatile(true))
	if err != nil {
		err = fmt.Errorf("can't analyse %s: %w", naming.SecHb, err)
		return
	}
	if b, err = store.DecodeKey("current_version"); err != nil {
		return
	} else if version, err = strconv.ParseUint(string(bytes.TrimSuffix(b, []byte("\n"))), 10, 64); err != nil {
		err = fmt.Errorf("convert current version %s to uint64: %w", string(b), err)
		return
	}
	if b, err = store.DecodeKey("current_secret"); err != nil {
		return
	} else {
		key = string(b)
	}

	if b, err = store.DecodeKey("next_version"); err != nil {
		return
	} else if altVersion, err = strconv.ParseUint(string(bytes.TrimSuffix(b, []byte("\n"))), 10, 64); err != nil {
		err = fmt.Errorf("convert next version %s to uint64: %w", string(b), err)
		return
	}
	if b, err = store.DecodeKey("next_secret"); err != nil {
		return
	} else {
		altKey = string(b)
	}
	sec = hbsecret.NewSecret(key, altKey, version, altVersion)
	return
}

func Set(prefix string, version uint64, secret string) error {
	if store, err := object.NewSec(naming.SecHb, object.WithVolatile(false)); err != nil {
		return err
	} else if err := store.TransactionChangeKey(prefix+"_secret", []byte(secret)); err != nil {
		return err
	} else if err := store.TransactionChangeKey(prefix+"_version", []byte(fmt.Sprintf("%d", version))); err != nil {
		return err
	} else if err := store.Config().Commit(); err != nil {
		return err
	}
	return nil
}
