package naming

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

type (
	// PathPattern is the objects of a namespace and a kind a pattern of
	// their name names, as "./sec/haproxy-*": the name holds one *, which
	// matches one character or more, the part of the name it matches
	// telling the objects apart.
	PathPattern struct {
		Namespace string
		Kind      Kind
		Name      string
	}
)

var (
	ErrPathPatternStars = errors.New("a name pattern holds one *")
)

// IsPathPattern says whether s names objects by a pattern of their name
// rather than one object.
func IsPathPattern(s string) bool {
	return strings.Contains(s, "*")
}

// ParsePathPatternRel parses the objects s names by a pattern of their name,
// relative to the namespace ns as ParsePathRel parses a path:
// "./sec/haproxy-*", "ns1/cfg/web-*", "sec/ca-*" in the root namespace.
func ParsePathPatternRel(s, ns string) (PathPattern, error) {
	var p PathPattern
	l := strings.Split(s, Separator)
	switch len(l) {
	case 3:
		p.Namespace, p.Name = l[0], l[2]
		if p.Namespace == "." {
			p.Namespace = ns
		}
	case 2:
		p.Namespace, p.Name = NsRoot, l[1]
	default:
		return p, fmt.Errorf("%w: %s: a name pattern is <namespace>/<kind>/<name>", ErrInvalid, s)
	}
	if strings.Count(p.Name, "*") != 1 {
		return p, fmt.Errorf("%w: %s: %w", ErrInvalid, s, ErrPathPatternStars)
	}
	// The pattern is valid when a name it matches is: its * replaced by
	// one of them.
	path, err := NewPathFromStrings(p.Namespace, l[len(l)-2], strings.Replace(p.Name, "*", "a", 1))
	if err != nil {
		return p, fmt.Errorf("%s: %w", s, err)
	}
	p.Kind = path.Kind
	return p, nil
}

func (t PathPattern) String() string {
	return Path{Namespace: t.Namespace, Kind: t.Kind, Name: t.Name}.String()
}

// Match returns the part of the name the * of the pattern matches, none when
// the name does not match it. The part is not empty: a * matching nothing
// would tell no object apart.
func (t PathPattern) Match(name string) (string, bool) {
	prefix, suffix, _ := strings.Cut(t.Name, "*")
	if len(name) <= len(prefix)+len(suffix) {
		return "", false
	}
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return "", false
	}
	return name[len(prefix) : len(name)-len(suffix)], true
}

// MatchPath says whether the pattern names the object at p.
func (t PathPattern) MatchPath(p Path) bool {
	if p.Namespace != t.Namespace || p.Kind != t.Kind {
		return false
	}
	_, ok := t.Match(p.Name)
	return ok
}

// InstalledPaths returns the objects the pattern names whose configuration
// is installed on the node, sorted.
func (t PathPattern) InstalledPaths() Paths {
	l := make(Paths, 0)
	glob := Path{Namespace: t.Namespace, Kind: t.Kind, Name: t.Name}.ConfigFile()
	matches, err := filepath.Glob(glob)
	if err != nil {
		return l
	}
	slices.Sort(matches)
	for _, m := range matches {
		p, err := NewPath(t.Namespace, t.Kind, strings.TrimSuffix(filepath.Base(m), ".conf"))
		if err != nil || !t.MatchPath(p) {
			continue
		}
		l = append(l, p)
	}
	return l
}
