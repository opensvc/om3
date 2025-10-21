package hbsecret

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
)

func DecodeSecretAndVersions() (currentVersion uint64, currentSecret string, nextVersion uint64, nextSecret string, err error) {
	var (
		b []byte

		store object.DataStore
	)
	store, err = object.NewSec(naming.SecHb, object.WithVolatile(true))
	if err != nil {
		err = fmt.Errorf("can't analyse %s: %w", naming.SecHb, err)
		return
	}
	if b, err = store.DecodeKey("current_version"); err != nil {
		return
	} else if currentVersion, err = strconv.ParseUint(string(bytes.TrimSuffix(b, []byte("\n"))), 10, 64); err != nil {
		err = fmt.Errorf("convert current version %s to uint64: %w", string(b), err)
		return
	}
	if b, err = store.DecodeKey("current_secret"); err != nil {
		return
	} else {
		currentSecret = string(b)
	}

	if b, err = store.DecodeKey("next_version"); err != nil {
		return
	} else if nextVersion, err = strconv.ParseUint(string(bytes.TrimSuffix(b, []byte("\n"))), 10, 64); err != nil {
		err = fmt.Errorf("convert next version %s to uint64: %w", string(b), err)
		return
	}
	if b, err = store.DecodeKey("next_secret"); err != nil {
		return
	} else {
		nextSecret = string(b)
	}
	return
}

func UpdateHb(prefix string, version uint64, secret string) error {
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
