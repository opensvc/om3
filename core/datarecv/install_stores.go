package datarecv

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/volsignal"
	"github.com/opensvc/om3/v3/util/converters"
)

type (
	// patternLine is an install line naming its datastores by a pattern of
	// their name, as "./sec/haproxy-*", an item each, as the parse of the
	// install keyword found them: the line installs from each one matching,
	// a "*" of its destination replaced by the part of the name the "*" of
	// the pattern matched.
	patternLine struct {
		id          string
		pattern     naming.PathPattern
		fromPattern string
		dest        string
		required    bool
		signals     *volsignal.T
		matched     int
	}

	patternLines []patternLine
)

var (
	errStorePatternKind = errors.New("a datastore name pattern is of sec or cfg datastores")
	errStorePatternDest = errors.New("the destination of a line installing from a datastore name pattern holds one *, replaced by the part of the name the pattern * matched")
)

// parseStorePattern parses the datastores s names by a pattern, relative to
// the namespace as a datastore path is: "./sec/haproxy-*", "ns1/cfg/web-*".
func parseStorePattern(s, namespace string) (naming.PathPattern, error) {
	p, err := naming.ParsePathPatternRel(s, namespace)
	if err != nil {
		return p, err
	}
	switch p.Kind {
	case naming.KindSec, naming.KindCfg:
	default:
		return p, fmt.Errorf("%s: %w", s, errStorePatternKind)
	}
	return p, nil
}

// storePaths returns the datastores the pattern names, existing on the node,
// and sharing their keys with the namespace installing them.
func storePaths(pattern naming.PathPattern, namespace string) naming.Paths {
	l := make(naming.Paths, 0)
	for _, p := range pattern.InstalledPaths() {
		if p.Namespace != namespace {
			store, err := object.NewDataStore(p, object.WithVolatile(true))
			if err != nil || !store.Allow(namespace) {
				continue
			}
		}
		l = append(l, p)
	}
	return l
}

// expandDest returns the destination of the line for the part of a datastore
// name the pattern matched, none when it would make the * a "." or ".."
// element of the path, as the name "tenant-.." would of "/tenants/*/": the
// file of a datastore is installed where the line says, and no higher.
func expandDest(dest, match string) (string, bool) {
	for _, elem := range strings.Split(dest, "/") {
		if !strings.Contains(elem, "*") {
			continue
		}
		switch strings.Replace(elem, "*", match, 1) {
		case ".", "..":
			return "", false
		}
	}
	return strings.Replace(dest, "*", match, 1), true
}

// line returns the line of the pattern the install line id is the item of
// a datastore of, none for the item of a line naming its datastore.
func (t patternLines) line(id string) (patternLine, bool) {
	i := strings.LastIndex(id, "@")
	if i < 0 {
		return patternLine{}, false
	}
	for _, l := range t {
		if l.id == id[:i] {
			return l, true
		}
	}
	return patternLine{}, false
}

// ValidateInstall says whether the install lines of value are ones the
// install takes, as far as the lines naming datastores by a pattern go: their
// pattern, and the * of their destination, the files of the datastores
// having to be told apart.
func ValidateInstall(value string) error {
	i, err := converters.Shlex.Convert(value)
	if err != nil {
		return err
	}
	words, _ := i.([]string)
	for _, line := range Split(words) {
		idx := slices.Index(line, "from")
		if idx < 0 || idx+1 >= len(line) {
			continue
		}
		from := line[idx+1]
		if !naming.IsPathPattern(from) {
			continue
		}
		// The namespace is any: a relative pattern is checked as the
		// one of the object would be.
		if _, err := parseStorePattern(from, "ns"); err != nil {
			return fmt.Errorf("install %s: %w", line[0], err)
		}
		if strings.Count(line[0], "*") != 1 {
			return fmt.Errorf("install %s from %s: %w", line[0], from, errStorePatternDest)
		}
	}
	return nil
}
