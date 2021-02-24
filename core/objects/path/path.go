package path

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/objects/kinds"
)

type (
	// Type represents an opensvc object path-like identifier. Ex: ns1/svc/svc1
	Type struct {
		// Name is the name part of the path
		Name string
		// Namespace is the namespace part of the path
		Namespace string
		// Kind is the kinf part of the path
		Kind kinds.Type
	}
)

const (
	// Separator is the character separating a path's namespace, kind and name
	Separator = "/"

	hostnameRegexStringRFC952 = `^[a-zA-Z]([a-zA-Z0-9\-]+[\.]?)*[a-zA-Z0-9]$` // https://tools.ietf.org/html/rfc952
	fqdnRegexStringRFC1123    = `^([a-zA-Z0-9]{1}[a-zA-Z0-9_-]{0,62})(\.[a-zA-Z0-9_]{1}[a-zA-Z0-9_-]{0,62})*?(\.[a-zA-Z]{1}[a-zA-Z0-9]{0,62})\.?$`
)

var (

	// ErrPathInvalid is raised when the path allocator can not return a Path
	// because one of the path element is not valid.
	ErrPathInvalid = errors.New("invalid path")

	hostnameRegexRFC952 = regexp.MustCompile(hostnameRegexStringRFC952)
	fqdnRegexRFC1123    = regexp.MustCompile(fqdnRegexStringRFC1123)
)

// New allocates a new path type from its elements
func New(name string, namespace string, kind string) (Type, error) {
	var t Type
	name = strings.ToLower(name)
	namespace = strings.ToLower(namespace)
	kind = strings.ToLower(kind)
	if name == "" {
		return t, errors.Wrap(ErrPathInvalid, "name is empty")
	}
	if kind == "" {
		kind = "svc"
	}
	k := kinds.New(kind)
	if k == kinds.Invalid {
		return t, errors.Wrapf(ErrPathInvalid, "invalid kind %s", kind)
	}
	if namespace == "" {
		namespace = "root"
	}
	if kind == "" {
		kind = "svc"
	}
	if !hostnameRegexRFC952.MatchString(name) {
		return t, errors.Wrapf(ErrPathInvalid, "invalid name %s (rfc952)", kind)
	}
	if !hostnameRegexRFC952.MatchString(namespace) {
		return t, errors.Wrapf(ErrPathInvalid, "invalid namespace %s (rfc952)", kind)
	}
	t.Namespace = namespace
	t.Name = name
	t.Kind = k
	return t, nil
}

func (t Type) String() string {
	var s string
	if t.Kind == kinds.Invalid {
		return ""
	}
	if t.Namespace != "" && t.Namespace != "root" {
		s += t.Namespace + Separator
	}
	fmt.Printf("xx %d\n", t.Kind)
	if t.Kind != kinds.Svc || s != "" {
		s += t.Kind.String() + Separator
	}
	return s + t.Name
}

// Split returns a new path struct from a path string representation
func (t Type) Split(s string) (Type, error) {
	var (
		name      string
		namespace string
		kind      string
	)
	s = strings.ToLower(s)
	l := strings.Split(s, Separator)
	switch len(l) {
	case 3:
		namespace = l[0]
		kind = l[1]
		name = l[2]
	case 2:
		namespace = "root"
		kind = l[1]
		name = l[2]
	case 1:
		namespace = "root"
		kind = "svc"
		name = l[2]
	}
	return New(name, namespace, kind)
}
