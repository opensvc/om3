package datarecv

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	// EnvItem is an item of an environment keyword: a variable with its
	// value, NAME=value, or a variable read from the key of a datastore,
	// NAME from <path> key <key>. A * in the name of a reference stands for
	// the key: with a key pattern, it names a variable per key matching.
	EnvItem struct {
		Literal string
		Name    string
		Ref     KeyMeta
	}

	// ErrEnvSourceMissing is a datastore or a key an environment item
	// reads not existing, which a resource may run without.
	ErrEnvSourceMissing struct {
		Path naming.Path
		Key  string
	}
)

var (
	envNameRegexp   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	envNameStRegexp = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)?\*[A-Za-z0-9_]*$`)
)

func (t ErrEnvSourceMissing) Error() string {
	if t.Key == "" {
		return fmt.Sprintf("%s does not exist", t.Path)
	}
	return fmt.Sprintf("%s has no key matching %s", t.Path, t.Key)
}

// IsRef says the item reads a datastore.
func (t EnvItem) IsRef() bool {
	return !t.Ref.Path.IsZero()
}

// String returns the item as written.
func (t EnvItem) String() string {
	if !t.IsRef() {
		return t.Literal
	}
	return fmt.Sprintf("%s from %s key %s", t.Name, t.Ref.Path, t.Ref.Key)
}

// ParseEnvironment parses the items of an environment keyword, as the shell
// lexer splits it, the relative paths of its references in the namespace.
//
// A word followed by "from" starts a reference, which no literal does: a
// literal holds an "=", or is a bare name a container engine reads from its
// own environment.
func ParseEnvironment(items []string, namespace string) ([]EnvItem, error) {
	l := make([]EnvItem, 0, len(items))
	for i := 0; i < len(items); i++ {
		s := items[i]
		if strings.Contains(s, "=") || i+1 >= len(items) || items[i+1] != "from" {
			l = append(l, EnvItem{Literal: s})
			continue
		}
		if i+4 >= len(items) || items[i+3] != "key" {
			return nil, fmt.Errorf("%s from ...: expected '<name> from <path> key <key>'", s)
		}
		km, err := ParseKeyMetaRel(strings.Join(items[i+1:i+5], " "), namespace)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s, err)
		}
		switch km.Path.Kind {
		case naming.KindSec, naming.KindCfg:
		default:
			return nil, fmt.Errorf("%s from %s: a sec or a cfg is expected", s, km.Path)
		}
		isPattern := strings.ContainsAny(km.Key, "*?[")
		switch {
		case envNameRegexp.MatchString(s):
			if isPattern {
				return nil, fmt.Errorf("%s from %s key %s: a key pattern names a variable per key, a * in the name stands for it", s, km.Path, km.Key)
			}
		case envNameStRegexp.MatchString(s):
		default:
			return nil, fmt.Errorf("%s: not a variable name", s)
		}
		l = append(l, EnvItem{Name: s, Ref: km})
		i += 4
	}
	return l, nil
}

// Resolve returns the variables of a reference, NAME=value, read from its
// datastore, which the namespace reading it is allowed to, and whether they
// are secret, read from a sec.
func (t EnvItem) Resolve() (env []string, secret bool, err error) {
	if !t.IsRef() {
		return []string{t.Literal}, false, nil
	}
	if !t.Ref.Path.Exists() {
		return nil, false, ErrEnvSourceMissing{Path: t.Ref.Path}
	}
	ds, err := object.NewDataStore(t.Ref.Path, object.WithVolatile(true))
	if err != nil {
		return nil, false, err
	}
	if !ds.Allow(t.Ref.From) {
		return nil, false, fmt.Errorf("the %s namespace is not allowed to access %s keys", t.Ref.From, t.Ref.Path)
	}
	var keys []string
	switch {
	case strings.ContainsAny(t.Ref.Key, "*?["):
		// The keys a pattern matches, as an install line reads them,
		// in the same order at each action.
		if keys, err = ds.MatchingKeys(t.Ref.Key); err != nil {
			return nil, false, err
		}
		slices.Sort(keys)
	case ds.HasKey(t.Ref.Key):
		keys = []string{t.Ref.Key}
	}
	if len(keys) == 0 {
		return nil, false, ErrEnvSourceMissing{Path: t.Ref.Path, Key: t.Ref.Key}
	}
	for _, k := range keys {
		b, err := ds.DecodeKey(k)
		if err != nil {
			return nil, false, fmt.Errorf("%s key %s: %w", t.Ref.Path, k, err)
		}
		env = append(env, strings.Replace(t.Name, "*", k, 1)+"="+string(b))
	}
	return env, t.Ref.Path.Kind == naming.KindSec, nil
}

// Environment returns the variables of the items of an environment keyword,
// NAME=value, in the order they are written, so the last of a name wins, and
// the names of the ones read from a sec, whose values are secret.
// A datastore or a key not existing is passed to missing, which says whether
// it is an error, nil going on without it.
func Environment(items []string, namespace string, missing func(error) error) (env []string, secrets map[string]bool, err error) {
	parsed, err := ParseEnvironment(items, namespace)
	if err != nil {
		return nil, nil, err
	}
	secrets = make(map[string]bool)
	for _, item := range parsed {
		l, isSecret, err := item.Resolve()
		var e ErrEnvSourceMissing
		if errors.As(err, &e) && missing != nil {
			if err := missing(fmt.Errorf("%s: %w", item, err)); err != nil {
				return nil, nil, err
			}
			continue
		} else if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", item, err)
		}
		for _, s := range l {
			name, _, _ := strings.Cut(s, "=")
			// A name read once from a sec stays a secret, the value
			// of the sec showing before the one overriding it.
			if isSecret {
				secrets[name] = true
			}
		}
		env = append(env, l...)
	}
	return env, secrets, nil
}

// EnvironmentDatastores returns the datastores the items of an environment
// keyword read, each once.
func EnvironmentDatastores(items []string, namespace string) (naming.Paths, error) {
	parsed, err := ParseEnvironment(items, namespace)
	if err != nil {
		return nil, err
	}
	l := make(naming.Paths, 0)
	for _, item := range parsed {
		if item.IsRef() && !slices.Contains(l, item.Ref.Path) {
			l = append(l, item.Ref.Path)
		}
	}
	return l, nil
}
