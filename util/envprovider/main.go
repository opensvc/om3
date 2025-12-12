// Package envprovider implement function to construct env vars from
// sec or cfg env items
package envprovider

import (
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	decoder interface {
		DecodeKey(keyname string) ([]byte, error)
		MatchingKeys(match string) ([]string, error)
	}
)

// From return []string env from configs_environment or secrets_environment
// Examples:
//
//	From([]string{"FOO=cfg1/key1"}, "namespace1", "cfg")
//	From([]string{"FOO=sec1/key1"}, "namespace1", "sec")
func From(items []string, ns, kd string) (result []string, err error) {
	for _, item := range items {
		if item == "[]" {
			continue
		}
		envs, err := envVars(item, ns, kd)
		if err != nil {
			return nil, err
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

func getKeysDecoder(name, ns, kd string) (decoder, error) {
	if p, err := naming.NewPathFromStrings(ns, kd, name); err != nil {
		return nil, err
	} else if !p.Exists() {
		return nil, fmt.Errorf("object %s does not exists", p)
	} else if o, err := object.New(p); err != nil {
		return nil, err
	} else if do, ok := o.(decoder); !ok {
		return nil, fmt.Errorf("object %s is not a decoder", p)
	} else {
		return do, nil
	}
}

func getKeys(name, ns, kd, match string) (s []string, err error) {
	var o decoder
	var keys []string
	var value string
	if o, err = getKeysDecoder(name, ns, kd); err != nil {
		return nil, err
	}
	if keys, err = o.MatchingKeys(match); err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("object %s has no key matching '%s'", o, match)

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
	if o, err := getKeysDecoder(name, ns, kd); err != nil {
		return "", err
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
