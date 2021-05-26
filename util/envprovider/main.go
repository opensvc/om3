// Package envprovider implement function to construct env vars from
// sec or cfg env items
package envprovider

import (
	"fmt"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"strings"
)

type (
	decoder interface {
		Decode(object.OptsDecode) ([]byte, error)
		Keys(object.OptsKeys) ([]string, error)
		Exists() bool
	}
)

// From return []string env from configs_environment or secrets_environment
// Examples:
//    From([]string{"FOO=cfg1/key1"}, "namespace1", "cfg")
//    From([]string{"FOO=sec1/key1"}, "namespace1", "sec")
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
	if p, err := path.New(name, ns, kd); err != nil {
		return nil, err
	} else if o, ok := object.NewFromPath(p).(decoder); !ok {
		return nil, fmt.Errorf("unable to get decoder ns:'%v', kind:'%v', name:'%v'", ns, kd, name)
	} else if !o.Exists() {
		return nil, fmt.Errorf("'%v' doesn't exists", o)
	} else {
		return o, nil
	}
}

func getKeys(name, ns, kd, match string) (s []string, err error) {
	var o decoder
	var keys []string
	var value string
	if o, err = getKeysDecoder(name, ns, kd); err != nil {
		return nil, err
	}
	keysOptions := object.OptsKeys{
		Global: object.OptsGlobal{},
		Lock:   object.OptsLocking{},
		Match:  match,
	}
	if keys, err = o.Keys(keysOptions); err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no key found matching '%v' on object '%v'", match, o)

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
	decodeOption := object.OptsDecode{
		Global: object.OptsGlobal{},
		Lock:   object.OptsLocking{},
		Key:    key,
	}
	if b, err = o.Decode(decodeOption); err != nil {
		return "", fmt.Errorf("unable to decode key '%v' on object '%v'", key, o)
	}
	return string(b), nil
}
