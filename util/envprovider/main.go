// Package envprovider implement function to construct env vars from
// sec or cfg env items
package envprovider

import (
	"errors"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	decoder interface {
		DecodeKey(keyname string) ([]byte, error)
		MatchingKeys(match string) ([]string, error)
		HasKey(string) bool
	}

	ErrObjectNotExist struct {
		Path naming.Path
	}
	ErrKeyNotExist struct {
		Path naming.Path
		Key  string
	}
	ErrObjectNotDecoder struct {
		Path naming.Path
	}
)

func (t ErrObjectNotExist) Error() string {
	return fmt.Sprintf("object %s does not exists", t.Path)
}

func (t ErrObjectNotDecoder) Error() string {
	return fmt.Sprintf("object %s is not a decoder", t.Path)
}

func (t ErrKeyNotExist) Error() string {
	return fmt.Sprintf("object %s has no key matching '%s'", t.Path, t.Key)
}

type IgnoreOption struct {
	ignore   func(error) bool
	onIgnore func(error)
}

func IgnoreNotExist(onIgnore func(error)) IgnoreOption {
	o := IgnoreOption{ignore: func(err error) bool {
		var e ErrObjectNotExist
		return errors.As(err, &e)
	}}
	o.onIgnore = onIgnore
	return o
}

func IgnoreNotDecoder(onIgnore func(error)) IgnoreOption {
	o := IgnoreOption{ignore: func(err error) bool {
		var e ErrObjectNotDecoder
		return errors.As(err, &e)
	}}
	o.onIgnore = onIgnore
	return o
}

func IgnoreKeyNotExist(onIgnore func(error)) IgnoreOption {
	o := IgnoreOption{ignore: func(err error) bool {
		var e ErrKeyNotExist
		return errors.As(err, &e)
	}}
	o.onIgnore = onIgnore
	return o
}

func IgnoreExpected(onIgnore func(error)) IgnoreOption {
	o := IgnoreOption{ignore: func(err error) bool {
		var (
			e1 ErrObjectNotExist
			e2 ErrObjectNotDecoder
			e3 ErrKeyNotExist
		)
		return errors.As(err, &e1) || errors.As(err, &e2) || errors.As(err, &e3)
	}}
	o.onIgnore = onIgnore
	return o
}

func eachError(err error, fn func(error) error) error {
	if err == nil {
		return nil
	}
	type multi interface{ Unwrap() []error }
	if m, ok := err.(multi); ok {
		var errs error
		for _, e := range m.Unwrap() {
			err1 := eachError(e, fn)
			if err1 != nil {
				errs = errors.Join(errs, err1)
			}
		}
		return errs
	} else {
		return fn(err)
	}
}

func filterErrors(err error, ignore ...IgnoreOption) error {
	if len(ignore) == 0 {
		return err
	}
	return eachError(err, func(e error) error {
		for _, opt := range ignore {
			if opt.ignore(e) {
				if opt.onIgnore != nil {
					opt.onIgnore(e)
				}
				return nil
			}
		}
		return e
	})
}

func From(items []string, namespace, kind string, ignore ...IgnoreOption) ([]string, error) {
	result, err := from(items, namespace, kind)
	if err != nil {
		err = filterErrors(err, ignore...)
	}
	return result, err
}

// from return []string env from configs_environment or secrets_environment
// Examples:
//
//	From([]string{"FOO=cfg1/key1"}, "namespace1", "cfg")
//	From([]string{"FOO=sec1/key1"}, "namespace1", "sec")
func from(items []string, ns, kd string) (result []string, err error) {
	for _, item := range items {
		if item == "[]" {
			continue
		}
		envs, err1 := envVars(item, ns, kd)
		if err1 != nil {
			err = errors.Join(err, fmt.Errorf("from %s: %w", item, err1))
		}
		result = append(result, envs...)
	}
	return
}

func envVars(envItem, ns, kd string) (result []string, err error) {
	splitEnvItem := strings.Split(envItem, "=")
	switch len(splitEnvItem) {
	case 1:
		nameMatch := strings.SplitN(splitEnvItem[0], "/", 2)
		return getKeys(nameMatch[0], ns, kd, nameMatch[1])
	case 2:
		nameKey := strings.SplitN(splitEnvItem[1], "/", 2)
		if len(nameKey) == 2 {
			var value string
			if value, err = getKey(nameKey[0], ns, kd, nameKey[1]); err != nil {
				return
			}
			return []string{splitEnvItem[0] + "=" + value}, nil
		}
	}
	return
}

func getKeysDecoder(path naming.Path) (decoder, error) {
	if !path.Exists() {
		return nil, ErrObjectNotExist{Path: path}
	} else if o, err := object.New(path); err != nil {
		return nil, err
	} else if do, ok := o.(decoder); !ok {
		return nil, ErrObjectNotDecoder{Path: path}
	} else {
		return do, nil
	}
}

func getKeys(name, ns, kd, match string) (s []string, err error) {
	path, err := naming.NewPathFromStrings(ns, kd, name)
	if err != nil {
		return nil, err
	}
	var o decoder
	var keys []string
	var value string
	if o, err = getKeysDecoder(path); err != nil {
		return nil, err
	}
	if keys, err = o.MatchingKeys(match); err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, ErrKeyNotExist{Path: path, Key: match}
	}
	for _, key := range keys {
		if value, err = decodeKey(o, key); err != nil {
			return nil, err
		}
		s = append(s, key+"="+value)
	}
	return s, nil
}

func getKey(name, ns, kd, key string) (string, error) {
	path, err := naming.NewPathFromStrings(ns, kd, name)
	if err != nil {
		return "", err
	}
	if o, err := getKeysDecoder(path); err != nil {
		return "", err
	} else if !o.HasKey(key) {
		return "", ErrKeyNotExist{Path: path, Key: key}
	} else {
		return decodeKey(o, key)
	}
}

func decodeKey(o decoder, key string) (s string, err error) {
	var b []byte
	if b, err = o.DecodeKey(key); err != nil {
		return "", fmt.Errorf("object %s key %s decode: %w", o, key, err)
	}
	return string(b), nil
}
