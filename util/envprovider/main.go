// Package envprovider implement function to construct env vars from
// sec or cfg envitems
package envprovider

import (
	"fmt"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"strings"
)

type (
	decoder interface {
		Decode(object.OptsDecode) ([]byte, error)
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
	if len(splitEnvItem) == 2 {
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

func getDecoder(name, ns, kd string) (o decoder, err error) {
	var p path.T
	if p, err = path.New(name, ns, kd); err != nil {
		return nil, err
	}
	switch p.Kind {
	case kind.Cfg:
		o = object.NewCfg(p)
	case kind.Sec:
		o = object.NewSec(p)
	default:
		return nil, fmt.Errorf("unexpected kind '%v'" + p.Kind.String())
	}
	return
}

func getKey(name, ns, kd, key string) (s string, err error) {
	var o decoder
	var b []byte
	if o, err = getDecoder(name, ns, kd); err != nil {
		return "", err
	}
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
