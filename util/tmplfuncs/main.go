// Package tmplfuncs provides the generic functions of the templates a user
// writes, as ox template renders: strings, encodings, defaults, matching.
//
// It has no function reading the environment or running a command: a
// template is data, rendered with the rights of whoever renders it.
package tmplfuncs

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/danwakefield/fnmatch"
	"gopkg.in/yaml.v3"
)

// FuncMap returns the generic functions, under the names a template uses.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"contains":  strings.Contains,
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
		"lower":     strings.ToLower,
		"upper":     strings.ToUpper,
		"trim":      strings.TrimSpace,
		"trimPrefix": func(prefix, s string) string {
			return strings.TrimPrefix(s, prefix)
		},
		"trimSuffix": func(suffix, s string) string {
			return strings.TrimSuffix(s, suffix)
		},
		"replace": func(old, new, s string) string {
			return strings.ReplaceAll(s, old, new)
		},
		"split": func(sep, s string) []string {
			return strings.Split(s, sep)
		},
		"fields": strings.Fields,
		"join":   join,
		"quote":  quote,
		"indent": indent,
		"nindent": func(n int, s string) string {
			return "\n" + indent(n, s)
		},
		"default":   defaultValue,
		"toJSON":    toJSON,
		"fromJSON":  fromJSON,
		"toYAML":    toYAML,
		"b64enc":    b64enc,
		"b64dec":    b64dec,
		"sha256sum": sha256sum,
		"reMatch":   reMatch,
		"fnMatch":   fnMatch,
		"sortAlpha": sortAlpha,
	}
}

// join joins the items of a list, of strings or of anything printed as
// a template prints it, with sep.
func join(sep string, v any) (string, error) {
	l, err := toStrings(v)
	if err != nil {
		return "", err
	}
	return strings.Join(l, sep), nil
}

// quote returns s quoted as a Go or json string.
func quote(v any) string {
	return fmt.Sprintf("%q", fmt.Sprint(v))
}

// indent prefixes each line of s with n spaces.
func indent(n int, s string) string {
	pad := strings.Repeat(" ", n)
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}

// defaultValue returns v, or def when v is empty: nil, zero, an empty
// string, list or map.
func defaultValue(def, v any) any {
	if isEmpty(v) {
		return def
	}
	return v
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return rv.Len() == 0
	case reflect.Pointer, reflect.Interface:
		return rv.IsNil()
	default:
		return rv.IsZero()
	}
}

func toJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

func fromJSON(s string) (any, error) {
	var v any
	err := json.Unmarshal([]byte(s), &v)
	return v, err
}

func toYAML(v any) (string, error) {
	b, err := yaml.Marshal(v)
	return strings.TrimSuffix(string(b), "\n"), err
}

func b64enc(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func b64dec(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	return string(b), err
}

func sha256sum(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func reMatch(pattern, s string) (bool, error) {
	r, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	return r.MatchString(s), nil
}

func fnMatch(pattern, s string) bool {
	return fnmatch.Match(pattern, s, 0)
}

// sortAlpha returns the items of a list as strings, sorted.
func sortAlpha(v any) ([]string, error) {
	l, err := toStrings(v)
	if err != nil {
		return nil, err
	}
	slices.Sort(l)
	return l, nil
}

func toStrings(v any) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	if l, ok := v.([]string); ok {
		return slices.Clone(l), nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, fmt.Errorf("%T is not a list", v)
	}
	l := make([]string, rv.Len())
	for i := range l {
		l[i] = fmt.Sprint(rv.Index(i).Interface())
	}
	return l, nil
}
